package chain

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// RPCBlockSource wraps *Client as a BlockSource.
// It uses the existing per-block RPC calls (BlockWithTxs + BlockReceipts + Logs)
// for Range, and a WebSocket newHeads subscription (falling back to polling) for
// Follow with parent-hash reorg detection.
type RPCBlockSource struct {
	client *Client
	log    *slog.Logger
}

// NewRPCBlockSource creates an RPCBlockSource from an existing chain Client.
func NewRPCBlockSource(c *Client, log *slog.Logger) *RPCBlockSource {
	if log == nil {
		log = slog.Default()
	}
	return &RPCBlockSource{client: c, log: log}
}

// Name returns "rpc".
func (s *RPCBlockSource) Name() string { return "rpc" }

// Head returns the current chain head block number.
func (s *RPCBlockSource) Head(ctx context.Context) (uint64, error) {
	blk, err := s.client.Head(ctx)
	if err != nil {
		return 0, fmt.Errorf("rpc head: %w", err)
	}
	return uint64(blk.Number), nil
}

// fetchOne fetches a complete block (block + receipts + filtered logs) via RPC.
func (s *RPCBlockSource) fetchOne(ctx context.Context, n uint64, logAddrs []string) (*FetchedBlock, error) {
	block, err := s.client.BlockWithTxs(ctx, n)
	if err != nil {
		return nil, fmt.Errorf("fetch block %d: %w", n, err)
	}

	receipts, err := s.client.BlockReceipts(ctx, block.Hash)
	if err != nil {
		return nil, fmt.Errorf("fetch receipts for block %d: %w", n, err)
	}

	var logs []RPCLog
	if len(logAddrs) > 0 {
		logs, err = s.client.Logs(ctx, LogFilter{
			FromBlock: fmt.Sprintf("0x%x", n),
			ToBlock:   fmt.Sprintf("0x%x", n),
			Address:   logAddrs,
		})
		if err != nil {
			return nil, fmt.Errorf("fetch logs for block %d: %w", n, err)
		}
	}

	return &FetchedBlock{Block: block, Receipts: receipts, Logs: logs}, nil
}

// Range streams blocks in [from, to] inclusive, in order, over RPC.
func (s *RPCBlockSource) Range(
	ctx context.Context,
	from, to uint64,
	logAddrs []string,
) (<-chan *FetchedBlock, <-chan error) {
	out := make(chan *FetchedBlock)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		for n := from; n <= to; n++ {
			if ctx.Err() != nil {
				errc <- ctx.Err()
				return
			}
			fb, err := s.fetchOne(ctx, n, logAddrs)
			if err != nil {
				errc <- err
				return
			}
			select {
			case <-ctx.Done():
				errc <- ctx.Err()
				return
			case out <- fb:
			}
		}
	}()

	return out, errc
}

// Follow streams new blocks from startBlock until ctx is cancelled.
// It subscribes to newHeads when the underlying connection is a WebSocket and
// falls back to polling otherwise. Reorgs are detected by parent-hash mismatch.
func (s *RPCBlockSource) Follow(
	ctx context.Context,
	startBlock uint64,
	logAddrs []string,
) (<-chan *FetchedBlock, <-chan error) {
	out := make(chan *FetchedBlock)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		heads := make(chan *RPCBlock, 64)
		sub, err := s.client.SubscribeNewHeads(ctx, heads)
		if err != nil {
			s.log.WarnContext(ctx, "rpc follow: subscribe failed, polling", "err", err)
			if perr := s.followPoll(ctx, startBlock, logAddrs, out); perr != nil {
				errc <- perr
			}
			return
		}
		defer sub.Unsubscribe()

		lastHash := ""
		lastNum := uint64(0)
		for {
			select {
			case <-ctx.Done():
				errc <- ctx.Err()
				return
			case err := <-sub.Err():
				if err != nil {
					s.log.WarnContext(ctx, "rpc follow: subscription error, polling", "err", err)
					if perr := s.followPoll(ctx, lastNum+1, logAddrs, out); perr != nil {
						errc <- perr
					}
					return
				}
			case head := <-heads:
				n := uint64(head.Number)
				if n < startBlock {
					continue
				}
				fb, err := s.fetchOne(ctx, n, logAddrs)
				if err != nil {
					errc <- err
					return
				}
				if lastHash != "" && lastNum == n-1 && fb.Block.ParentHash.Hex() != lastHash {
					fb.Reorg = &ReorgInfo{
						CommonAncestor: n - 1,
						OldHash:        lastHash,
						NewHash:        fb.Block.Hash.Hex(),
						Depth:          1,
					}
				}
				lastHash = fb.Block.Hash.Hex()
				lastNum = n
				select {
				case <-ctx.Done():
					errc <- ctx.Err()
					return
				case out <- fb:
				}
			}
		}
	}()

	return out, errc
}

// followPoll polls the head every 250ms and emits new blocks.
func (s *RPCBlockSource) followPoll(
	ctx context.Context,
	startBlock uint64,
	logAddrs []string,
	out chan<- *FetchedBlock,
) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	lastHash := ""
	next := startBlock
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			head, err := s.client.Head(ctx)
			if err != nil {
				s.log.WarnContext(ctx, "rpc follow poll: head failed", "err", err)
				continue
			}
			headNum := uint64(head.Number)
			for n := next; n <= headNum; n++ {
				fb, err := s.fetchOne(ctx, n, logAddrs)
				if err != nil {
					return err
				}
				if lastHash != "" && fb.Block.ParentHash.Hex() != lastHash {
					fb.Reorg = &ReorgInfo{
						CommonAncestor: n - 1,
						OldHash:        lastHash,
						NewHash:        fb.Block.Hash.Hex(),
						Depth:          1,
					}
				}
				lastHash = fb.Block.Hash.Hex()
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- fb:
				}
			}
			next = headNum + 1
		}
	}
}
