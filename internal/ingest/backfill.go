package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/sync/errgroup"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
	"github.com/uwezukwechibuzor/priority-watch/internal/store"
)

// INSERT SQL constants matching the raw.* column order from migrations.
const (
	InsertBlockSQL = `INSERT INTO raw.blocks
		(block_number, block_hash, parent_hash, block_timestamp, l1_block_number,
		 base_fee_per_gas, gas_used, gas_limit, tx_count, size, _ingested_at, _version)`

	InsertTxSQL = `INSERT INTO raw.transactions
		(block_number, block_timestamp, tx_index, tx_hash, tx_type,
		 from_address, to_address, nonce, value, gas_limit, gas_price,
		 max_fee_per_gas, max_priority_fee_per_gas, input_selector, input_size,
		 _ingested_at, _version)`

	InsertReceiptSQL = `INSERT INTO raw.receipts
		(block_number, block_timestamp, tx_index, tx_hash, status,
		 gas_used, gas_used_for_l1, effective_gas_price, cumulative_gas_used,
		 contract_address, log_count, _ingested_at, _version)`

	InsertLogSQL = `INSERT INTO raw.logs
		(block_number, block_timestamp, tx_index, tx_hash, log_index,
		 address, topic0, topic1, topic2, topic3, data, _ingested_at, _version)`

	InsertReorgLogSQL = `INSERT INTO raw.reorg_log
		(detected_at, fork_block, old_hash, new_hash, depth)`

	InsertBalanceSnapshotSQL = `INSERT INTO raw.balance_snapshots
		(address, block_number, block_timestamp, balance_wei, _ingested_at, _version)`
)

// ChainClient is the minimal interface required by BackfillRunner.
type ChainClient interface {
	BlockHeader(ctx context.Context, number uint64) (*chain.RPCBlock, error)
	BlockWithTxs(ctx context.Context, number uint64) (*chain.RPCBlock, error)
	BlockReceipts(ctx context.Context, blockHash common.Hash) ([]chain.RPCReceipt, error)
	Logs(ctx context.Context, filter chain.LogFilter) ([]chain.RPCLog, error)
	Head(ctx context.Context) (*chain.RPCBlock, error)
}

// Writers holds the four batch writers for raw tables.
type Writers struct {
	Blocks   *store.BatchWriter
	Txs      *store.BatchWriter
	Receipts *store.BatchWriter
	Logs     *store.BatchWriter
}

// BackfillConfig holds parameters for a single backfill run.
type BackfillConfig struct {
	From         uint64
	To           uint64
	ChunkSize    uint64
	Concurrency  int
	LogAddresses []string // pool + contract addresses to filter logs; empty = all
	LogsOnly     bool     // only fetch and store logs; skip blocks/txs/receipts
}

// BackfillRunner performs a resumable, idempotent backfill of a block range.
// It uses an errgroup pipeline: one producer goroutine sends block numbers into
// a buffered channel; N worker goroutines fetch, validate, transform, and write.
type BackfillRunner struct {
	chain   ChainClient
	cursors *CursorStore
	writers *Writers
	metrics *BackfillMetrics
	log     *slog.Logger
}

// NewBackfillRunner creates a BackfillRunner.
func NewBackfillRunner(
	c ChainClient,
	cursors *CursorStore,
	writers *Writers,
	metrics *BackfillMetrics,
	log *slog.Logger,
) *BackfillRunner {
	return &BackfillRunner{
		chain:   c,
		cursors: cursors,
		writers: writers,
		metrics: metrics,
		log:     log,
	}
}

// Run executes the backfill for the range in cfg.
// It plans chunks, skips already-done chunks (resume after crash), and
// processes each remaining chunk with a concurrent pipeline.
// Graceful shutdown: cancel ctx (e.g. via signal.NotifyContext) to stop
// after the current block finishes.
func (r *BackfillRunner) Run(ctx context.Context, cfg BackfillConfig) error {
	if cfg.ChunkSize == 0 {
		cfg.ChunkSize = DefaultChunkSize
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 4
	}

	chunks := PlanChunks(cfg.From, cfg.To, cfg.ChunkSize)

	// Register all chunks (idempotent — ReplacingMergeTree deduplicates).
	for _, c := range chunks {
		if err := r.cursors.Register(ctx, c); err != nil {
			return fmt.Errorf("register chunks: %w", err)
		}
	}

	// Load already-completed chunks to skip them.
	done, err := r.cursors.LoadDone(ctx)
	if err != nil {
		return fmt.Errorf("load done chunks: %w", err)
	}

	for _, c := range chunks {
		if _, skip := done[cursorKey(r.cursors.jobPrefix, c)]; skip {
			r.log.InfoContext(ctx, "skipping done chunk", "from", c.From, "to", c.To)
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := r.runChunk(ctx, c, cfg); err != nil {
			return fmt.Errorf("chunk %d-%d: %w", c.From, c.To, err)
		}
	}
	return nil
}

// runChunk processes one chunk with the producer→workers pipeline.
func (r *BackfillRunner) runChunk(ctx context.Context, c Chunk, cfg BackfillConfig) error {
	start := time.Now()

	if err := r.cursors.SetRunning(ctx, c, 1); err != nil {
		r.log.WarnContext(ctx, "set running failed", "err", err)
	}

	blockNums := make(chan uint64, cfg.Concurrency*2)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(cfg.Concurrency + 1) // N workers + 1 producer

	// Producer: sends block numbers [from..to] into blockNums.
	eg.Go(func() error {
		defer close(blockNums)
		for n := c.From; n <= c.To; n++ {
			select {
			case <-egCtx.Done():
				return egCtx.Err()
			case blockNums <- n:
			}
		}
		return nil
	})

	// Workers: each goroutine consumes from blockNums.
	for range cfg.Concurrency {
		eg.Go(func() error {
			return r.worker(egCtx, c, blockNums, cfg.LogAddresses, cfg.LogsOnly)
		})
	}

	if err := eg.Wait(); err != nil {
		_ = r.cursors.SetFailed(ctx, c, 1, err.Error())
		return err
	}

	elapsed := time.Since(start).Seconds()
	blocks := float64(c.To - c.From + 1)
	if r.metrics != nil {
		r.metrics.ChunkDuration.Observe(elapsed)
		if elapsed > 0 {
			r.metrics.BlocksPerSecond.Set(blocks / elapsed)
		}
	}

	if err := r.cursors.SetDone(ctx, c); err != nil {
		r.log.WarnContext(ctx, "set done failed", "err", err)
	}

	r.log.InfoContext(ctx, "chunk done",
		"from", c.From, "to", c.To,
		"blocks", int(blocks), "elapsed_s", fmt.Sprintf("%.1f", elapsed),
	)
	return nil
}

// worker consumes block numbers from nums, fetches each block's data,
// validates it, transforms it, and appends rows to the batch writers.
func (r *BackfillRunner) worker(
	ctx context.Context,
	c Chunk,
	nums <-chan uint64,
	logAddresses []string,
	logsOnly bool,
) error {
	now := time.Now().UTC()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case n, ok := <-nums:
			if !ok {
				return nil
			}
			if err := Block(ctx, r.chain, r.writers, n, logAddresses, now, logsOnly); err != nil {
				return fmt.Errorf("block %d: %w", n, err)
			}
			if r.metrics != nil {
				r.metrics.BlocksIngested.Add(1)
			}
			_ = r.cursors.SetProgress(ctx, c, n, 1)
		}
	}
}

// Block fetches, validates, transforms and writes one block to the four raw tables.
// It is called by BackfillRunner and the live Follower.
// When logsOnly is true, only raw.logs is written (blocks/txs/receipts are skipped).
func Block(
	ctx context.Context,
	c ChainClient,
	w *Writers,
	n uint64,
	logAddresses []string,
	now time.Time,
	logsOnly bool,
) error {
	block, err := c.BlockWithTxs(ctx, n)
	if err != nil {
		return fmt.Errorf("fetch block: %w", err)
	}

	var receipts []chain.RPCReceipt
	if !logsOnly {
		receipts, err = c.BlockReceipts(ctx, block.Hash)
		if err != nil {
			return fmt.Errorf("fetch receipts: %w", err)
		}
		if err := ValidateBlock(block, receipts); err != nil {
			return err
		}
	}

	var logs []chain.RPCLog
	if len(logAddresses) > 0 {
		logs, err = c.Logs(ctx, chain.LogFilter{
			FromBlock: fmt.Sprintf("0x%x", n),
			ToBlock:   fmt.Sprintf("0x%x", n),
			Address:   logAddresses,
		})
		if err != nil {
			return fmt.Errorf("fetch logs: %w", err)
		}
	}

	blk, txs, rcpts, evts := TransformBlock(block, receipts, logs, now)

	if !logsOnly {
		if err := appendBlockRow(ctx, w.Blocks, blk); err != nil {
			return fmt.Errorf("append block: %w", err)
		}
		for _, tx := range txs {
			if err := appendTxRow(ctx, w.Txs, tx); err != nil {
				return fmt.Errorf("append tx: %w", err)
			}
		}
		for _, rc := range rcpts {
			if err := appendReceiptRow(ctx, w.Receipts, rc); err != nil {
				return fmt.Errorf("append receipt: %w", err)
			}
		}
	}
	for _, ev := range evts {
		if err := appendLogRow(ctx, w.Logs, ev); err != nil {
			return fmt.Errorf("append log: %w", err)
		}
	}
	return nil
}

// appendBlockRow writes one BlockRow to w.
func appendBlockRow(ctx context.Context, w *store.BatchWriter, b BlockRow) error {
	return w.Append(ctx,
		b.BlockNumber, b.BlockHash, b.ParentHash,
		b.BlockTimestamp, b.L1BlockNumber, b.BaseFeePerGas,
		b.GasUsed, b.GasLimit, b.TxCount, b.Size,
		b.IngestedAt, b.Version,
	)
}

// appendTxRow writes one TxRow to w.
func appendTxRow(ctx context.Context, w *store.BatchWriter, t TxRow) error {
	return w.Append(ctx,
		t.BlockNumber, t.BlockTimestamp, t.TxIndex, t.TxHash, t.TxType,
		t.FromAddress, t.ToAddress, t.Nonce, t.Value, t.GasLimit, t.GasPrice,
		t.MaxFeePerGas, t.MaxPriorityFeePerGas, t.InputSelector, t.InputSize,
		t.IngestedAt, t.Version,
	)
}

// appendReceiptRow writes one ReceiptRow to w.
func appendReceiptRow(ctx context.Context, w *store.BatchWriter, rc ReceiptRow) error {
	return w.Append(ctx,
		rc.BlockNumber, rc.BlockTimestamp, rc.TxIndex, rc.TxHash, rc.Status,
		rc.GasUsed, rc.GasUsedForL1, rc.EffectiveGasPrice, rc.CumulativeGasUsed,
		rc.ContractAddress, rc.LogCount,
		rc.IngestedAt, rc.Version,
	)
}

// appendLogRow writes one LogRow to w.
func appendLogRow(ctx context.Context, w *store.BatchWriter, l LogRow) error {
	return w.Append(ctx,
		l.BlockNumber, l.BlockTimestamp, l.TxIndex, l.TxHash, l.LogIndex,
		l.Address, l.Topic0, l.Topic1, l.Topic2, l.Topic3, l.Data,
		l.IngestedAt, l.Version,
	)
}
