package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
)

// BlockTimestampQuerier fetches only the block header (no txs) to get its timestamp.
type BlockTimestampQuerier interface {
	HeadNumber(ctx context.Context) (uint64, error)
	BlockTimestamp(ctx context.Context, number uint64) (uint64, error)
}

// DateResolver resolves dates to block numbers using binary search on timestamps.
type DateResolver struct {
	q BlockTimestampQuerier
}

// NewDateResolver creates a DateResolver backed by q.
func NewDateResolver(q BlockTimestampQuerier) *DateResolver {
	return &DateResolver{q: q}
}

// ChainClientQuerier adapts a *chain.Client to the BlockTimestampQuerier interface.
// It uses BlockHeader (no transactions) for efficiency.
type ChainClientQuerier struct {
	c interface {
		Head(ctx context.Context) (*chain.RPCBlock, error)
		BlockHeader(ctx context.Context, number uint64) (*chain.RPCBlock, error)
	}
}

// NewChainClientQuerier wraps a chain.Client as a BlockTimestampQuerier.
func NewChainClientQuerier(c interface {
	Head(ctx context.Context) (*chain.RPCBlock, error)
	BlockHeader(ctx context.Context, number uint64) (*chain.RPCBlock, error)
}) *ChainClientQuerier {
	return &ChainClientQuerier{c: c}
}

func (a *ChainClientQuerier) HeadNumber(ctx context.Context) (uint64, error) {
	blk, err := a.c.Head(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(blk.Number), nil
}

func (a *ChainClientQuerier) BlockTimestamp(ctx context.Context, n uint64) (uint64, error) {
	blk, err := a.c.BlockHeader(ctx, n)
	if err != nil {
		return 0, fmt.Errorf("block header %d: %w", n, err)
	}
	return uint64(blk.Timestamp), nil
}

// Resolve returns the first block whose timestamp is >= t.
// If t is after the latest block, it returns the latest block number.
func (r *DateResolver) Resolve(ctx context.Context, t time.Time) (uint64, error) {
	target := uint64(t.Unix())

	head, err := r.q.HeadNumber(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolve date: get head: %w", err)
	}

	// Fast-path: if the head is before the target, clamp to head.
	headTS, err := r.q.BlockTimestamp(ctx, head)
	if err != nil {
		return 0, fmt.Errorf("resolve date: head timestamp: %w", err)
	}
	if headTS <= target {
		return head, nil
	}

	// Binary search over [0, head] for the smallest block with timestamp >= target.
	lo, hi := uint64(0), head
	for lo < hi {
		mid := (lo + hi) / 2
		ts, err := r.q.BlockTimestamp(ctx, mid)
		if err != nil {
			return 0, fmt.Errorf("resolve date: block %d timestamp: %w", mid, err)
		}
		if ts < target {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, nil
}
