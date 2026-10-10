package models

import "context"

// Opportunity is one arb event the simulator can replay.
type Opportunity struct {
	BlockNumber   uint64
	EstProfitGwei float64 // estimated gross arb profit (gwei)
	ActualTopFee  float64 // winning fee in that block (gwei)
	BaseFeeGwei   float64 // EIP-1559 base fee (gwei)
}

// ReplayResult holds the aggregate outcome of a backtest run.
type ReplayResult struct {
	Strategy   string    `json:"strategy"`
	TotalPnL   float64   `json:"total_pnl_gwei"`
	WinRate    float64   `json:"win_rate"`
	Wins       int       `json:"wins"`
	Total      int       `json:"total"`
	AvgCost    float64   `json:"avg_cost_gwei"`
	CumPnL     []float64 `json:"cumulative_pnl_gwei"`
	Disclaimer string    `json:"disclaimer"`
}

// Strategy is the interface every bidding strategy must satisfy.
type Strategy interface {
	// Name returns the strategy identifier used in API responses.
	Name() string
	// Tip returns the priority fee (gwei/gas) to bid for a given opportunity.
	// recent is the fee history available at the time of the opportunity.
	Tip(ctx context.Context, opp Opportunity, recent []BlockFeeRecord) (float64, error)
}

// AllStrategies returns one instance of every built-in strategy.
func AllStrategies() []Strategy {
	return []Strategy{
		NewFixedTip(1.0),
		NewPercentileOfRecent(0.90),
		NewSpreadProportional(0.50),
	}
}

const replayDisclaimer = "Competitor reactions are not modelled: this replay " +
	"assumes your bid does not change rivals' behaviour, which overstates " +
	"the strategy's real-world win rate and P&L."
