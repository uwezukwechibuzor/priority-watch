package chain

import (
	"context"
	"errors"
)

// errNoBlocks is returned internally by FailoverBlockSource when a source
// completed a range without producing any blocks, triggering a fallback.
var errNoBlocks = errors.New("chain: source produced no blocks")

// FetchedBlock contains a complete block with all data, ready for TransformBlock.
// Reorg is non-nil when this event signals a chain reorganisation.
type FetchedBlock struct {
	Block         *RPCBlock
	Receipts      []RPCReceipt
	Logs          []RPCLog // pre-filtered by log address config
	Reorg         *ReorgInfo
	FinalizedHead uint64 // 0 if unknown
}

// ReorgInfo describes a detected chain reorganisation.
type ReorgInfo struct {
	CommonAncestor uint64
	OldHash        string
	NewHash        string
	Depth          uint32
}

// BlockSource is the abstraction over chain data providers.
// Implementations: RPCBlockSource, SQDBlockSource, HyperSyncBlockSource, FailoverBlockSource.
type BlockSource interface {
	// Range streams blocks in [from, to] inclusive, in order.
	// The error channel receives at most one error; both channels are closed when done.
	Range(ctx context.Context, from, to uint64, logAddrs []string) (<-chan *FetchedBlock, <-chan error)
	// Follow streams new blocks from startBlock until ctx is cancelled.
	// On reorg, FetchedBlock.Reorg is set; the block is the new canonical tip.
	Follow(ctx context.Context, startBlock uint64, logAddrs []string) (<-chan *FetchedBlock, <-chan error)
	// Head returns the current chain head block number.
	Head(ctx context.Context) (uint64, error)
	// Name returns the provider name for logging ("rpc", "sqd", "hypersync").
	Name() string
}
