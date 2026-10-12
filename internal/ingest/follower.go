package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
	"github.com/uwezukwechibuzor/priority-watch/internal/store"
)

// FollowerChainClient extends ChainClient with methods only the follower needs.
type FollowerChainClient interface {
	ChainClient
	Safe(ctx context.Context) (*chain.RPCBlock, error)
	Finalized(ctx context.Context) (*chain.RPCBlock, error)
	Balance(ctx context.Context, addr common.Address, blockTag string) (*big.Int, error)
}

// wsSubscriber is the minimal interface for WebSocket subscriptions.
// Satisfied by *chain.Client when the underlying connection is a WebSocket.
type wsSubscriber interface {
	SubscribeNewHeads(ctx context.Context, ch chan<- *chain.RPCBlock) (chain.HeadSubscription, error)
}

// FollowerWriters holds all batch writers the follower uses.
type FollowerWriters struct {
	Blocks       *store.BatchWriter
	Txs          *store.BatchWriter
	Receipts     *store.BatchWriter
	Logs         *store.BatchWriter
	ReorgLog     *store.BatchWriter
	Balances     *store.BatchWriter
	StatusUpsert *store.SourceStatusUpsert
}

// blockRef is an (number, hash) pair kept in the reorg detection window.
type blockRef struct {
	Number uint64
	Hash   string
}

// reorgWindow is a bounded FIFO of blockRef values used for reorg detection.
type reorgWindow struct {
	entries []blockRef
	maxSize int
}

func newReorgWindow(maxSize int) *reorgWindow {
	return &reorgWindow{maxSize: maxSize}
}

// push appends b; evicts the oldest entry when full.
func (w *reorgWindow) push(b blockRef) {
	if len(w.entries) >= w.maxSize {
		w.entries = w.entries[1:]
	}
	w.entries = append(w.entries, b)
}

// last returns the most recent blockRef, or zero value if empty.
func (w *reorgWindow) last() (blockRef, bool) {
	if len(w.entries) == 0 {
		return blockRef{}, false
	}
	return w.entries[len(w.entries)-1], true
}

// lastNumber returns the highest block number in the window, or 0.
func (w *reorgWindow) lastNumber() uint64 {
	ref, ok := w.last()
	if !ok {
		return 0
	}
	return ref.Number
}

// findByHash returns the index of the entry whose Hash == hash, or -1.
func (w *reorgWindow) findByHash(hash string) int {
	for i, e := range w.entries {
		if e.Hash == hash {
			return i
		}
	}
	return -1
}

// truncateAfter removes all entries after index i (inclusive of i), keeping [0..i].
func (w *reorgWindow) truncateAfter(i int) {
	if i < 0 || i >= len(w.entries) {
		return
	}
	w.entries = w.entries[:i+1]
}

// FollowerConfig holds tuning parameters for Follower.
type FollowerConfig struct {
	LogAddresses    []string
	GapThreshold    uint64
	WindowSize      int
	PollInterval    time.Duration
	BalanceInterval time.Duration
	SafeInterval    time.Duration
	FeeRecipients   []common.Address
}

func (c *FollowerConfig) setDefaults() {
	if c.WindowSize <= 0 {
		c.WindowSize = 512
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 250 * time.Millisecond
	}
	if c.BalanceInterval <= 0 {
		c.BalanceInterval = time.Hour
	}
	if c.SafeInterval <= 0 {
		c.SafeInterval = 30 * time.Second
	}
	if c.GapThreshold == 0 {
		c.GapThreshold = 1000
	}
}

// Follower subscribes to new block heads and ingests each block in real time.
type Follower struct {
	chain   FollowerChainClient
	ws      wsSubscriber
	cursors *CursorStore
	writers *FollowerWriters
	metrics *FollowerMetrics
	cfg     FollowerConfig
	window  *reorgWindow
	log     *slog.Logger

	lastWrite time.Time
}

// NewFollower creates a Follower. ws may be nil; polling is used in that case.
func NewFollower(
	c FollowerChainClient,
	ws wsSubscriber,
	cursors *CursorStore,
	writers *FollowerWriters,
	metrics *FollowerMetrics,
	cfg FollowerConfig,
	log *slog.Logger,
) *Follower {
	cfg.setDefaults()
	return &Follower{
		chain:   c,
		ws:      ws,
		cursors: cursors,
		writers: writers,
		metrics: metrics,
		cfg:     cfg,
		window:  newReorgWindow(cfg.WindowSize),
		log:     log,
	}
}

// HealthFunc returns a health-check function for obs.Server.SetHealthCheck.
// Unhealthy if no write has occurred in the last 30 s or the head lag exceeds 60 s.
func (f *Follower) HealthFunc() func() error {
	return func() error {
		if !f.lastWrite.IsZero() && time.Since(f.lastWrite) > 30*time.Second {
			return fmt.Errorf("no write in %s", time.Since(f.lastWrite).Truncate(time.Second))
		}
		return nil
	}
}

// Run starts the follower loop. It blocks until ctx is cancelled.
func (f *Follower) Run(ctx context.Context) error {
	if err := f.startup(ctx); err != nil {
		return fmt.Errorf("follower startup: %w", err)
	}

	go f.runSafeFinalized(ctx)
	go f.runBalanceSnapshots(ctx)

	if f.ws != nil {
		return f.runWS(ctx)
	}
	return f.runPoll(ctx)
}

// startup resolves the highest stored block and pre-loads the reorg window.
func (f *Follower) startup(ctx context.Context) error {
	head, err := f.chain.Head(ctx)
	if err != nil {
		return fmt.Errorf("get head: %w", err)
	}
	chainHead := uint64(head.Number)

	lastStored, err := f.cursors.HighestDoneBlock(ctx)
	if err != nil {
		f.log.WarnContext(ctx, "highest done block query failed, assuming 0", "err", err)
		lastStored = 0
	}
	// Fallback: if no cursor history exists, use the max block already in raw.blocks
	// to avoid re-enqueuing the entire history on a fresh follower start.
	if lastStored == 0 {
		if maxRaw, ferr := f.cursors.MaxRawBlock(ctx); ferr == nil && maxRaw > 0 {
			lastStored = maxRaw
		}
	}

	const maxEnqueueBlocks = 200_000 // cap to avoid flooding meta.ingest_cursors
	if chainHead > lastStored && chainHead-lastStored > f.cfg.GapThreshold {
		from := lastStored + 1
		// If the gap is huge (e.g. first-ever start) only enqueue the recent tail;
		// a manual backfill via pw-ingester backfill handles deep history.
		if chainHead-from > maxEnqueueBlocks {
			f.log.WarnContext(ctx, "gap too large for auto-enqueue; enqueuing recent tail only; run 'pw-ingester backfill' for full history",
				"from", from, "to", chainHead, "gap", chainHead-from,
				"enqueue_from", chainHead-maxEnqueueBlocks)
			from = chainHead - maxEnqueueBlocks
		}
		f.log.InfoContext(ctx, "gap exceeds threshold, enqueuing backfill chunks",
			"from", from, "to", chainHead, "gap", chainHead-lastStored)
		if err := f.enqueueBackfillChunks(ctx, from, chainHead); err != nil {
			f.log.WarnContext(ctx, "enqueue backfill failed", "err", err)
		}
	}

	// Pre-load window with the last windowSize block headers.
	windowStart := chainHead
	windowSize := uint64(f.cfg.WindowSize) //nolint:gosec
	if chainHead >= windowSize {
		windowStart = chainHead - windowSize + 1
	}
	for n := windowStart; n <= chainHead; n++ {
		blk, err := f.chain.BlockHeader(ctx, n)
		if err != nil {
			f.log.WarnContext(ctx, "window preload failed", "block", n, "err", err)
			break
		}
		f.window.push(blockRef{Number: n, Hash: blk.Hash.String()})
	}

	f.log.InfoContext(ctx, "follower started",
		"chain_head", chainHead, "window_size", len(f.window.entries),
	)
	return nil
}

// runWS drives the follower via WebSocket subscription.
func (f *Follower) runWS(ctx context.Context) error {
	ch := make(chan *chain.RPCBlock, 64)
	sub, err := f.ws.SubscribeNewHeads(ctx, ch)
	if err != nil {
		f.log.WarnContext(ctx, "WS subscribe failed, falling back to poll", "err", err)
		return f.runPoll(ctx)
	}
	defer sub.Unsubscribe()

	f.log.InfoContext(ctx, "following via WebSocket newHeads subscription")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-sub.Err():
			if err != nil {
				f.log.WarnContext(ctx, "WS subscription error, falling back to poll", "err", err)
				return f.runPoll(ctx)
			}
		case blk := <-ch:
			if err := f.processBlock(ctx, blk); err != nil {
				f.log.ErrorContext(ctx, "process block error", "block", uint64(blk.Number), "err", err)
			}
		}
	}
}

// runPoll drives the follower by polling the head every PollInterval.
func (f *Follower) runPoll(ctx context.Context) error {
	f.log.InfoContext(ctx, "following via polling", "interval", f.cfg.PollInterval)

	ticker := time.NewTicker(f.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			blk, err := f.chain.Head(ctx)
			if err != nil {
				f.log.WarnContext(ctx, "poll head failed", "err", err)
				continue
			}
			n := uint64(blk.Number)
			if n <= f.window.lastNumber() {
				continue
			}
			if err := f.processBlock(ctx, blk); err != nil {
				f.log.ErrorContext(ctx, "process block error", "block", n, "err", err)
			}
		}
	}
}

// processBlock checks parent-hash continuity and ingests the block.
func (f *Follower) processBlock(ctx context.Context, blk *chain.RPCBlock) error {
	n := uint64(blk.Number)
	parentHash := blk.ParentHash.String()

	if last, ok := f.window.last(); ok && last.Number == n-1 {
		if last.Hash != parentHash {
			ancestorIdx := f.window.findByHash(parentHash)
			if err := f.handleReorg(ctx, blk, ancestorIdx); err != nil {
				return fmt.Errorf("handle reorg at block %d: %w", n, err)
			}
			return nil
		}
	}

	return f.ingestAndPush(ctx, blk)
}

// handleReorg logs the reorg, truncates the window, and re-ingests the canonical chain.
func (f *Follower) handleReorg(ctx context.Context, tip *chain.RPCBlock, ancestorIdx int) error {
	n := uint64(tip.Number)

	forkBlock := n
	oldHash := ""
	depth := uint32(f.cfg.WindowSize) //nolint:gosec

	if ancestorIdx >= 0 && ancestorIdx < len(f.window.entries) {
		ancestor := f.window.entries[ancestorIdx]
		forkBlock = ancestor.Number + 1
		if ancestorIdx+1 < len(f.window.entries) {
			oldHash = f.window.entries[ancestorIdx+1].Hash
		}
		depth = uint32(len(f.window.entries) - 1 - ancestorIdx) //nolint:gosec
	} else if last, ok := f.window.last(); ok {
		oldHash = last.Hash
	}

	f.log.WarnContext(ctx, "reorg detected",
		"block", n, "fork_block", forkBlock,
		"old_hash", oldHash, "new_hash", tip.Hash.String(), "depth", depth,
	)

	if f.writers.ReorgLog != nil {
		now := time.Now().UTC()
		if err := f.writers.ReorgLog.Append(ctx,
			now, forkBlock, oldHash, tip.Hash.String(), depth,
		); err != nil {
			f.log.WarnContext(ctx, "reorg log append failed", "err", err)
		}
	}

	if f.metrics != nil {
		f.metrics.ReorgsTotal.Add(1)
	}

	if ancestorIdx >= 0 {
		f.window.truncateAfter(ancestorIdx)
	} else {
		f.window.entries = nil
	}

	w := &Writers{
		Blocks:   f.writers.Blocks,
		Txs:      f.writers.Txs,
		Receipts: f.writers.Receipts,
		Logs:     f.writers.Logs,
	}
	for reIngest := forkBlock; reIngest <= n; reIngest++ {
		fresh, err := f.chain.BlockHeader(ctx, reIngest)
		if err != nil {
			return fmt.Errorf("re-fetch block %d: %w", reIngest, err)
		}
		if err := Block(ctx, f.chain, w, reIngest, f.cfg.LogAddresses, time.Now().UTC(), false); err != nil {
			return fmt.Errorf("re-ingest block %d: %w", reIngest, err)
		}
		f.window.push(blockRef{Number: reIngest, Hash: fresh.Hash.String()})
	}

	f.lastWrite = time.Now().UTC()
	return nil
}

// ingestAndPush ingests blk, updates metrics, and pushes to the reorg window.
func (f *Follower) ingestAndPush(ctx context.Context, blk *chain.RPCBlock) error {
	n := uint64(blk.Number)
	now := time.Now().UTC()

	w := &Writers{
		Blocks:   f.writers.Blocks,
		Txs:      f.writers.Txs,
		Receipts: f.writers.Receipts,
		Logs:     f.writers.Logs,
	}
	if err := Block(ctx, f.chain, w, n, f.cfg.LogAddresses, now, false); err != nil {
		return fmt.Errorf("ingest block %d: %w", n, err)
	}

	f.window.push(blockRef{Number: n, Hash: blk.Hash.String()})
	f.lastWrite = now

	if f.metrics != nil {
		f.metrics.HeadBlock.Set(float64(n))
		ts := time.Unix(int64(blk.Timestamp), 0) //nolint:gosec
		f.metrics.IngestLagSeconds.Set(time.Since(ts).Seconds())
	}

	if f.writers.StatusUpsert != nil {
		_ = f.writers.StatusUpsert.Upsert(ctx, store.SourceStatusRow{
			Source:          "follower",
			Rows:            1,
			LatestPosition:  strconv.FormatUint(n, 10),
			LatestTimestamp: time.Unix(int64(blk.Timestamp), 0), //nolint:gosec
			UpdatedAt:       now,
		})
	}

	return nil
}

// runSafeFinalized periodically refreshes safe/finalized lag metrics.
func (f *Follower) runSafeFinalized(ctx context.Context) {
	ticker := time.NewTicker(f.cfg.SafeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.updateSafeFinalized(ctx)
		}
	}
}

func (f *Follower) updateSafeFinalized(ctx context.Context) {
	if f.metrics == nil {
		return
	}
	head := f.window.lastNumber()
	if safe, err := f.chain.Safe(ctx); err == nil {
		s := uint64(safe.Number)
		if head >= s {
			f.metrics.SafeLagBlocks.Set(float64(head - s))
		}
	}
	if fin, err := f.chain.Finalized(ctx); err == nil {
		fn := uint64(fin.Number)
		if head >= fn {
			f.metrics.FinalizedLagBlocks.Set(float64(head - fn))
		}
	}
}

// runBalanceSnapshots periodically snapshots fee-recipient balances.
func (f *Follower) runBalanceSnapshots(ctx context.Context) {
	ticker := time.NewTicker(f.cfg.BalanceInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.snapshotBalances(ctx)
		}
	}
}

func (f *Follower) snapshotBalances(ctx context.Context) {
	if len(f.cfg.FeeRecipients) == 0 || f.writers.Balances == nil {
		return
	}
	head, err := f.chain.Head(ctx)
	if err != nil {
		f.log.WarnContext(ctx, "balance snapshot: get head failed", "err", err)
		return
	}
	n := uint64(head.Number)
	ts := time.Unix(int64(head.Timestamp), 0) //nolint:gosec
	now := time.Now().UTC()

	for _, addr := range f.cfg.FeeRecipients {
		bal, err := f.chain.Balance(ctx, addr, "latest")
		if err != nil {
			f.log.WarnContext(ctx, "balance fetch failed", "addr", addr, "err", err)
			bal = new(big.Int)
		}
		if err := f.writers.Balances.Append(ctx,
			addr.Hex(), n, ts, bal, now, uint64(now.UnixMilli()),
		); err != nil {
			f.log.WarnContext(ctx, "balance snapshot append failed",
				"addr", addr, "err", err)
		}
	}
}

// enqueueBackfillChunks registers cursor rows for the range so the backfill
// worker or Airflow DAG can catch up.
func (f *Follower) enqueueBackfillChunks(ctx context.Context, from, to uint64) error {
	chunks := PlanChunks(from, to, DefaultChunkSize)
	var errs []error
	for _, c := range chunks {
		if err := f.cursors.Register(ctx, c); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
