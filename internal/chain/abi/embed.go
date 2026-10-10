// Package abi contains embedded ABI JSON files and typed log decoders for
// the events Priority Watch tracks.
//
// # Sources
//
//   - uniswap_v3_pool.json — Uniswap/v3-core@d8b1c635c275d2a9450bd6a78f3fa2484fef73eb
//     contracts/interfaces/pool/IUniswapV3PoolEvents.sol
//   - algebra_pool.json    — cryptoalgebra/Algebra@c03fc1c6fd071fce9308a169d3dc44fc87b72f6e
//     src/core/contracts/interfaces/pool/IAlgebraPoolEvents.sol (Camelot V3)
//   - aave_v3_pool.json    — aave/aave-v3-core@782f51917056a53a2c228701058a6c3fb233684a
//     contracts/interfaces/IPool.sol
//   - erc20.json           — EIP-20 standard (https://eips.ethereum.org/EIPS/eip-20)
//   - timeboost_auction.json — OffchainLabs/nitro-contracts@11f6b90bd3322992c36d9f0549f82a6f92e14e6f
//     src/express-lane-auction/IExpressLaneAuction.sol
package abi

import (
	"embed"
	"fmt"
	"math/big"
	"strings"

	gethABI "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// FS is the embedded filesystem containing all ABI JSON files.
//
//go:embed *.json
var FS embed.FS

// Loaded ABIs — parsed once at init.
var (
	UniswapV3Pool    gethABI.ABI
	AlgebraPool      gethABI.ABI
	AaveV3Pool       gethABI.ABI
	ERC20            gethABI.ABI
	TimeboostAuction gethABI.ABI
)

// Event topic0 hashes — used for fast log routing without ABI parsing.
var (
	TopicUniswapV3Swap            common.Hash
	TopicAlgebraSwap              common.Hash
	TopicAaveV3LiquidationCall    common.Hash
	TopicERC20Transfer            common.Hash
	TopicTimeboostAuctionResolved common.Hash
	TopicTimeboostDeposit         common.Hash
)

func init() {
	loadABI := func(name string) gethABI.ABI {
		data, err := FS.ReadFile(name)
		if err != nil {
			panic(fmt.Sprintf("abi: cannot read %s: %v", name, err))
		}
		a, err := gethABI.JSON(strings.NewReader(string(data)))
		if err != nil {
			panic(fmt.Sprintf("abi: cannot parse %s: %v", name, err))
		}
		return a
	}

	UniswapV3Pool = loadABI("uniswap_v3_pool.json")
	AlgebraPool = loadABI("algebra_pool.json")
	AaveV3Pool = loadABI("aave_v3_pool.json")
	ERC20 = loadABI("erc20.json")
	TimeboostAuction = loadABI("timeboost_auction.json")

	// Compute topic0 hashes from parsed ABIs.
	TopicUniswapV3Swap = UniswapV3Pool.Events["Swap"].ID
	TopicAlgebraSwap = AlgebraPool.Events["Swap"].ID
	TopicAaveV3LiquidationCall = AaveV3Pool.Events["LiquidationCall"].ID
	TopicERC20Transfer = ERC20.Events["Transfer"].ID
	TopicTimeboostAuctionResolved = TimeboostAuction.Events["AuctionResolved"].ID
	TopicTimeboostDeposit = TimeboostAuction.Events["Deposit"].ID
}

// ── Typed decoded structs ────────────────────────────────────────────────────

// UniswapV3SwapEvent contains decoded fields from a Uniswap V3 Swap log.
type UniswapV3SwapEvent struct {
	Sender       common.Address
	Recipient    common.Address
	Amount0      *big.Int
	Amount1      *big.Int
	SqrtPriceX96 *big.Int
	Liquidity    *big.Int
	Tick         int32
}

// AlgebraSwapEvent contains decoded fields from an Algebra (Camelot V3) Swap log.
type AlgebraSwapEvent struct {
	Sender      common.Address
	Recipient   common.Address
	Amount0     *big.Int
	Amount1     *big.Int
	Price       *big.Int
	Liquidity   *big.Int
	Tick        int32
	OverrideFee uint32
	PluginFee   uint32
}

// AaveV3LiquidationCallEvent contains decoded fields from an Aave V3 LiquidationCall log.
type AaveV3LiquidationCallEvent struct {
	CollateralAsset            common.Address
	DebtAsset                  common.Address
	User                       common.Address
	DebtToCover                *big.Int
	LiquidatedCollateralAmount *big.Int
	Liquidator                 common.Address
	ReceiveAToken              bool
}

// ERC20TransferEvent contains decoded fields from an ERC-20 Transfer log.
type ERC20TransferEvent struct {
	From  common.Address
	To    common.Address
	Value *big.Int
}

// TimeboostAuctionResolvedEvent contains decoded fields from an AuctionResolved log.
type TimeboostAuctionResolvedEvent struct {
	IsMultiBidAuction               bool
	Round                           uint64
	FirstPriceBidder                common.Address
	FirstPriceExpressLaneController common.Address
	FirstPriceAmount                *big.Int
	Price                           *big.Int
	RoundStartTimestamp             uint64
	RoundEndTimestamp               uint64
}

// TimeboostDepositEvent contains decoded fields from a Deposit log.
type TimeboostDepositEvent struct {
	Account common.Address
	Amount  *big.Int
}

// ── Decoders ──────────────────────────────────────────────────────────────────

// DecodeUniswapV3Swap decodes a raw log into a UniswapV3SwapEvent.
// The caller should verify log.Topics[0] == TopicUniswapV3Swap first.
func DecodeUniswapV3Swap(topics []common.Hash, data []byte) (*UniswapV3SwapEvent, error) {
	out := &UniswapV3SwapEvent{}
	if len(topics) < 3 {
		return nil, fmt.Errorf("uniswap v3 swap: need 3 topics, got %d", len(topics))
	}
	out.Sender = common.BytesToAddress(topics[1].Bytes())
	out.Recipient = common.BytesToAddress(topics[2].Bytes())

	args, err := UniswapV3Pool.Events["Swap"].Inputs.NonIndexed().Unpack(data)
	if err != nil {
		return nil, fmt.Errorf("uniswap v3 swap unpack: %w", err)
	}
	out.Amount0, _ = args[0].(*big.Int)
	out.Amount1, _ = args[1].(*big.Int)
	out.SqrtPriceX96, _ = args[2].(*big.Int)
	out.Liquidity, _ = args[3].(*big.Int)
	if tick, ok := args[4].(int32); ok {
		out.Tick = tick
	} else if tick8, ok := args[4].(int8); ok {
		out.Tick = int32(tick8)
	}
	return out, nil
}

// DecodeAlgebraSwap decodes a raw log into an AlgebraSwapEvent.
func DecodeAlgebraSwap(topics []common.Hash, data []byte) (*AlgebraSwapEvent, error) {
	out := &AlgebraSwapEvent{}
	if len(topics) < 3 {
		return nil, fmt.Errorf("algebra swap: need 3 topics, got %d", len(topics))
	}
	out.Sender = common.BytesToAddress(topics[1].Bytes())
	out.Recipient = common.BytesToAddress(topics[2].Bytes())

	args, err := AlgebraPool.Events["Swap"].Inputs.NonIndexed().Unpack(data)
	if err != nil {
		return nil, fmt.Errorf("algebra swap unpack: %w", err)
	}
	out.Amount0, _ = args[0].(*big.Int)
	out.Amount1, _ = args[1].(*big.Int)
	out.Price, _ = args[2].(*big.Int)
	out.Liquidity, _ = args[3].(*big.Int)
	if tick, ok := args[4].(int32); ok {
		out.Tick = tick
	} else if tick8, ok := args[4].(int8); ok {
		out.Tick = int32(tick8)
	}
	if fee, ok := args[5].(uint32); ok {
		out.OverrideFee = fee
	}
	if fee, ok := args[6].(uint32); ok {
		out.PluginFee = fee
	}
	return out, nil
}

// DecodeAaveV3LiquidationCall decodes a raw log into an AaveV3LiquidationCallEvent.
func DecodeAaveV3LiquidationCall(topics []common.Hash, data []byte) (*AaveV3LiquidationCallEvent, error) {
	out := &AaveV3LiquidationCallEvent{}
	if len(topics) < 4 {
		return nil, fmt.Errorf("aave liquidation call: need 4 topics, got %d", len(topics))
	}
	out.CollateralAsset = common.BytesToAddress(topics[1].Bytes())
	out.DebtAsset = common.BytesToAddress(topics[2].Bytes())
	out.User = common.BytesToAddress(topics[3].Bytes())

	args, err := AaveV3Pool.Events["LiquidationCall"].Inputs.NonIndexed().Unpack(data)
	if err != nil {
		return nil, fmt.Errorf("aave liquidation call unpack: %w", err)
	}
	out.DebtToCover, _ = args[0].(*big.Int)
	out.LiquidatedCollateralAmount, _ = args[1].(*big.Int)
	out.Liquidator, _ = args[2].(common.Address)
	out.ReceiveAToken, _ = args[3].(bool)
	return out, nil
}

// DecodeERC20Transfer decodes a raw log into an ERC20TransferEvent.
func DecodeERC20Transfer(topics []common.Hash, data []byte) (*ERC20TransferEvent, error) {
	out := &ERC20TransferEvent{}
	if len(topics) < 3 {
		return nil, fmt.Errorf("erc20 transfer: need 3 topics, got %d", len(topics))
	}
	out.From = common.BytesToAddress(topics[1].Bytes())
	out.To = common.BytesToAddress(topics[2].Bytes())

	args, err := ERC20.Events["Transfer"].Inputs.NonIndexed().Unpack(data)
	if err != nil {
		return nil, fmt.Errorf("erc20 transfer unpack: %w", err)
	}
	out.Value, _ = args[0].(*big.Int)
	return out, nil
}

// DecodeTimeboostAuctionResolved decodes a raw log into a TimeboostAuctionResolvedEvent.
func DecodeTimeboostAuctionResolved(topics []common.Hash, data []byte) (*TimeboostAuctionResolvedEvent, error) {
	out := &TimeboostAuctionResolvedEvent{}
	if len(topics) < 4 {
		return nil, fmt.Errorf("timeboost auction resolved: need 4 topics, got %d", len(topics))
	}
	// indexed: isMultiBidAuction (bool), firstPriceBidder (address), firstPriceExpressLaneController (address)
	if topics[1] != (common.Hash{}) {
		out.IsMultiBidAuction = topics[1].Big().Sign() != 0
	}
	out.FirstPriceBidder = common.BytesToAddress(topics[2].Bytes())
	out.FirstPriceExpressLaneController = common.BytesToAddress(topics[3].Bytes())

	args, err := TimeboostAuction.Events["AuctionResolved"].Inputs.NonIndexed().Unpack(data)
	if err != nil {
		return nil, fmt.Errorf("timeboost auction resolved unpack: %w", err)
	}
	if v, ok := args[0].(uint64); ok {
		out.Round = v
	}
	out.FirstPriceAmount, _ = args[1].(*big.Int)
	out.Price, _ = args[2].(*big.Int)
	if v, ok := args[3].(uint64); ok {
		out.RoundStartTimestamp = v
	}
	if v, ok := args[4].(uint64); ok {
		out.RoundEndTimestamp = v
	}
	return out, nil
}

// DecodeTimeboostDeposit decodes a raw log into a TimeboostDepositEvent.
func DecodeTimeboostDeposit(topics []common.Hash, data []byte) (*TimeboostDepositEvent, error) {
	out := &TimeboostDepositEvent{}
	if len(topics) < 2 {
		return nil, fmt.Errorf("timeboost deposit: need 2 topics, got %d", len(topics))
	}
	out.Account = common.BytesToAddress(topics[1].Bytes())

	args, err := TimeboostAuction.Events["Deposit"].Inputs.NonIndexed().Unpack(data)
	if err != nil {
		return nil, fmt.Errorf("timeboost deposit unpack: %w", err)
	}
	out.Amount, _ = args[0].(*big.Int)
	return out, nil
}
