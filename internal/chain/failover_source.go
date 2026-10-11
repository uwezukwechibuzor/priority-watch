package chain

import (
	"context"
	"log/slog"
)

// FailoverBlockSource tries primary, then falls back to secondary on error.
type FailoverBlockSource struct {
	primary   BlockSource
	secondary BlockSource
	log       *slog.Logger
}

// NewFailoverBlockSource wraps a primary and secondary BlockSource.
func NewFailoverBlockSource(primary, secondary BlockSource, log *slog.Logger) *FailoverBlockSource {
	if log == nil {
		log = slog.Default()
	}
	return &FailoverBlockSource{primary: primary, secondary: secondary, log: log}
}

// Name returns "failover(<primary>+<secondary>)".
func (s *FailoverBlockSource) Name() string {
	return "failover(" + s.primary.Name() + "+" + s.secondary.Name() + ")"
}

// Head returns the primary head, falling back to the secondary on error.
func (s *FailoverBlockSource) Head(ctx context.Context) (uint64, error) {
	h, err := s.primary.Head(ctx)
	if err != nil {
		s.log.WarnContext(ctx, "failover: primary head failed, using secondary",
			"primary", s.primary.Name(), "err", err)
		return s.secondary.Head(ctx)
	}
	return h, nil
}

// Range streams from primary; if the primary errors before producing a block,
// it restarts the range on the secondary. Blocks already emitted by the primary
// are re-emitted from the secondary only when the primary produced none.
func (s *FailoverBlockSource) Range(
	ctx context.Context,
	from, to uint64,
	logAddrs []string,
) (<-chan *FetchedBlock, <-chan error) {
	out := make(chan *FetchedBlock)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		if s.drainRange(ctx, s.primary, from, to, logAddrs, out) == nil {
			return
		}
		if ctx.Err() != nil {
			errc <- ctx.Err()
			return
		}
		s.log.WarnContext(ctx, "failover: primary range failed, retrying with secondary",
			"primary", s.primary.Name(), "secondary", s.secondary.Name())
		if err := s.drainRange(ctx, s.secondary, from, to, logAddrs, out); err != nil {
			errc <- err
		}
	}()

	return out, errc
}

// drainRange reads a full range from src into out. It returns an error if src
// errored or produced zero blocks (so the caller can fall back to the secondary).
func (s *FailoverBlockSource) drainRange(
	ctx context.Context,
	src BlockSource,
	from, to uint64,
	logAddrs []string,
	out chan<- *FetchedBlock,
) error {
	blocks, errs := src.Range(ctx, from, to, logAddrs)
	var count uint64
	for fb := range blocks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- fb:
			count++
		}
	}
	if err := <-errs; err != nil {
		return err
	}
	if count == 0 {
		return errNoBlocks
	}
	return nil
}

// Follow streams from the primary; on error it switches to the secondary.
func (s *FailoverBlockSource) Follow(
	ctx context.Context,
	startBlock uint64,
	logAddrs []string,
) (<-chan *FetchedBlock, <-chan error) {
	out := make(chan *FetchedBlock)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		next := startBlock
		if last, err := s.drainFollow(ctx, s.primary, next, logAddrs, out); err != nil {
			if ctx.Err() != nil {
				errc <- ctx.Err()
				return
			}
			if last >= next {
				next = last + 1
			}
			s.log.WarnContext(ctx, "failover: primary follow failed, switching to secondary",
				"primary", s.primary.Name(), "secondary", s.secondary.Name(), "err", err)
			if _, serr := s.drainFollow(ctx, s.secondary, next, logAddrs, out); serr != nil && ctx.Err() == nil {
				errc <- serr
			}
		}
	}()

	return out, errc
}

// drainFollow reads from src.Follow into out until the source errors or ctx ends.
// It returns the last emitted block number and the terminating error.
func (s *FailoverBlockSource) drainFollow(
	ctx context.Context,
	src BlockSource,
	startBlock uint64,
	logAddrs []string,
	out chan<- *FetchedBlock,
) (uint64, error) {
	blocks, errs := src.Follow(ctx, startBlock, logAddrs)
	var last uint64
	for fb := range blocks {
		if fb.Block != nil {
			last = uint64(fb.Block.Number)
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case out <- fb:
		}
	}
	return last, <-errs
}
