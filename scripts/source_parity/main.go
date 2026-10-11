// Command source_parity fetches the same block range from every configured
// data source (RPC, SQD Portal, Envio HyperSync), compares every column of the
// resulting rows, and prints a Markdown agreement table.
//
// It exits non-zero if any column mismatches. When no sources are configured it
// prints a hint and exits 0, so it is safe to run in CI without credentials.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain/hypersync"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain/sqd"
	"github.com/uwezukwechibuzor/priority-watch/internal/ingest"
)

// Mismatch records a single column disagreement between two sources.
type Mismatch struct {
	BlockNumber uint64
	Column      string
	SourceA     string
	ValueA      string
	SourceB     string
	ValueB      string
}

// ParityReport summarises a parity comparison run.
type ParityReport struct {
	FromBlock     uint64
	ToBlock       uint64
	BlocksChecked uint64
	Mismatches    []Mismatch
}

func main() {
	var (
		from  uint64
		count uint64
	)
	flag.Uint64Var(&from, "from", 0, "first block to compare")
	flag.Uint64Var(&count, "count", 2000, "number of blocks to compare")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	sources := configuredSources(log)
	if len(sources) < 2 {
		fmt.Println("source_parity: fewer than two sources configured.")
		fmt.Println("Set ARBITRUM_RPC_URL and SQD_PORTAL_URL (and optionally ENVIO_HYPERSYNC_URL)")
		fmt.Println("to run the parity check, then re-run with --from <block> --count <n>.")
		return
	}
	if from == 0 {
		fmt.Println("source_parity: --from not set; nothing to compare. Pass --from <block>.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	report, err := run(ctx, sources, from, from+count-1)
	if err != nil {
		log.Error("parity run failed", "err", err)
		os.Exit(1)
	}
	printReport(report, sources)
	if len(report.Mismatches) > 0 {
		os.Exit(1)
	}
}

// namedSource pairs a BlockSource with its display name.
type namedSource struct {
	name string
	src  chain.BlockSource
}

// configuredSources returns the sources that are configured via env vars.
func configuredSources(log *slog.Logger) []namedSource {
	var out []namedSource
	if url := os.Getenv("SQD_PORTAL_URL"); url != "" {
		out = append(out, namedSource{"sqd", sqd.New(url, log)})
	}
	if url := os.Getenv("ENVIO_HYPERSYNC_URL"); url != "" {
		out = append(out, namedSource{"hypersync", hypersync.New(url, os.Getenv("ENVIO_API_TOKEN"), log)})
	}
	// RPC is handled separately: it needs a dialled client. Left as a documented
	// extension point; add chain.New + chain.NewRPCBlockSource here when a reliable
	// archive RPC is available.
	return out
}

// run fetches [from,to] from every source and compares block-level columns.
func run(ctx context.Context, sources []namedSource, from, to uint64) (*ParityReport, error) {
	rowsBySource := make(map[string]map[uint64]ingest.BlockRow)
	now := time.Now().UTC()

	for _, s := range sources {
		rows := make(map[uint64]ingest.BlockRow)
		blocks, errc := s.src.Range(ctx, from, to, nil)
		for fb := range blocks {
			blk, _, _, _ := ingest.TransformBlock(fb.Block, fb.Receipts, fb.Logs, now)
			rows[blk.BlockNumber] = blk
		}
		if err := <-errc; err != nil {
			return nil, fmt.Errorf("%s range: %w", s.name, err)
		}
		rowsBySource[s.name] = rows
	}

	report := &ParityReport{FromBlock: from, ToBlock: to}
	for n := from; n <= to; n++ {
		report.BlocksChecked++
		for i := 0; i < len(sources); i++ {
			for j := i + 1; j < len(sources); j++ {
				a, b := sources[i].name, sources[j].name
				ra, oka := rowsBySource[a][n]
				rb, okb := rowsBySource[b][n]
				if !oka || !okb {
					continue
				}
				report.Mismatches = append(report.Mismatches, compareBlock(n, a, ra, b, rb)...)
			}
		}
	}
	return report, nil
}

// compareBlock compares the load-bearing columns of two BlockRows.
func compareBlock(n uint64, a string, ra ingest.BlockRow, b string, rb ingest.BlockRow) []Mismatch {
	var out []Mismatch
	add := func(col, va, vb string) {
		if va != vb {
			out = append(out, Mismatch{BlockNumber: n, Column: col, SourceA: a, ValueA: va, SourceB: b, ValueB: vb})
		}
	}
	add("block_hash", ra.BlockHash, rb.BlockHash)
	add("parent_hash", ra.ParentHash, rb.ParentHash)
	add("gas_used", fmt.Sprint(ra.GasUsed), fmt.Sprint(rb.GasUsed))
	add("gas_limit", fmt.Sprint(ra.GasLimit), fmt.Sprint(rb.GasLimit))
	add("base_fee_per_gas", ra.BaseFeePerGas.String(), rb.BaseFeePerGas.String())
	add("tx_count", fmt.Sprint(ra.TxCount), fmt.Sprint(rb.TxCount))
	add("l1_block_number", fmt.Sprint(ra.L1BlockNumber), fmt.Sprint(rb.L1BlockNumber))
	return out
}

// printReport renders a Markdown summary of the parity comparison.
func printReport(r *ParityReport, sources []namedSource) {
	names := make([]string, 0, len(sources))
	for _, s := range sources {
		names = append(names, s.name)
	}
	fmt.Printf("# Source parity report\n\n")
	fmt.Printf("- Sources: %v\n", names)
	fmt.Printf("- Range: %d – %d (%d blocks)\n", r.FromBlock, r.ToBlock, r.BlocksChecked)
	fmt.Printf("- Mismatches: %d\n\n", len(r.Mismatches))

	if len(r.Mismatches) == 0 {
		fmt.Println("All compared columns agree across sources.")
		return
	}

	byCol := make(map[string]int)
	for _, m := range r.Mismatches {
		byCol[m.Column]++
	}
	cols := make([]string, 0, len(byCol))
	for c := range byCol {
		cols = append(cols, c)
	}
	sort.Strings(cols)

	fmt.Println("| Column | Mismatches |")
	fmt.Println("|--------|-----------|")
	for _, c := range cols {
		fmt.Printf("| %s | %d |\n", c, byCol[c])
	}
	fmt.Println()
	fmt.Println("## First mismatches")
	fmt.Println()
	fmt.Println("| Block | Column | A | ValueA | B | ValueB |")
	fmt.Println("|-------|--------|---|--------|---|--------|")
	limit := 20
	for i, m := range r.Mismatches {
		if i >= limit {
			break
		}
		fmt.Printf("| %d | %s | %s | %s | %s | %s |\n",
			m.BlockNumber, m.Column, m.SourceA, m.ValueA, m.SourceB, m.ValueB)
	}
}
