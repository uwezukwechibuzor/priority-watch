package cex

// DefaultSymbols maps common token symbols to their Binance trading pair symbols.
// Used when building the list of streams to subscribe to or backfill.
var DefaultSymbols = map[string]string{
	"WETH": "ETHUSDT",
	"ETH":  "ETHUSDT",
	"WBTC": "BTCUSDT",
	"BTC":  "BTCUSDT",
	"ARB":  "ARBUSDT",
}

// Stablecoins is the set of token symbols treated as exactly 1.00 USD.
// They are excluded from CEX price feeds; callers substitute 1.0 directly.
var Stablecoins = map[string]bool{
	"USDC":  true,
	"USDT":  true,
	"DAI":   true,
	"FRAX":  true,
	"LUSD":  true,
	"USDCE": true, // bridged USDC on Arbitrum
}
