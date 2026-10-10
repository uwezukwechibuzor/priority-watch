package abi_test

import (
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/go-cmp/cmp"
	chainabi "github.com/uwezukwechibuzor/priority-watch/internal/chain/abi"
)

// ── ERC-20 Transfer ─────────────────────────────────────────────────────────

func TestDecodeERC20Transfer(t *testing.T) {
	t.Parallel()

	from := common.HexToAddress("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	to := common.HexToAddress("0xcafecafecafecafecafecafecafecafecafecafe")
	value := big.NewInt(1_000_000_000_000_000_000) // 1 ETH

	// Encode the non-indexed data: value padded to 32 bytes.
	data := common.LeftPadBytes(value.Bytes(), 32)
	topics := []common.Hash{
		chainabi.TopicERC20Transfer,
		common.BytesToHash(from.Bytes()),
		common.BytesToHash(to.Bytes()),
	}

	tests := []struct {
		name    string
		topics  []common.Hash
		data    []byte
		wantErr bool
	}{
		{
			name:    "valid transfer",
			topics:  topics,
			data:    data,
			wantErr: false,
		},
		{
			name:    "too few topics",
			topics:  topics[:2],
			data:    data,
			wantErr: true,
		},
		{
			name:    "empty data still decodes zero value",
			topics:  topics,
			data:    common.LeftPadBytes([]byte{0}, 32),
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ev, err := chainabi.DecodeERC20Transfer(tc.topics, tc.data)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.name == "valid transfer" {
				if diff := cmp.Diff(from, ev.From); diff != "" {
					t.Errorf("From mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(to, ev.To); diff != "" {
					t.Errorf("To mismatch (-want +got):\n%s", diff)
				}
				if ev.Value.Cmp(value) != 0 {
					t.Errorf("Value = %s, want %s", ev.Value, value)
				}
			}
		})
	}
}

// ── Uniswap V3 Swap ─────────────────────────────────────────────────────────

func TestDecodeUniswapV3Swap(t *testing.T) {
	t.Parallel()

	sender := common.HexToAddress("0x1111111111111111111111111111111111111111")
	recipient := common.HexToAddress("0x2222222222222222222222222222222222222222")

	// Non-indexed: amount0, amount1, sqrtPriceX96, liquidity, tick
	// Each int256/uint256 is 32 bytes; int24 is also padded to 32 bytes.
	amount0 := big.NewInt(-1000000)
	amount1 := big.NewInt(2000000)
	sqrtPrice := new(big.Int).SetUint64(1_000_000_000_000)
	liquidity := new(big.Int).SetUint64(5_000_000)
	tick := int32(887272)

	data := encodeSwapData(amount0, amount1, sqrtPrice, liquidity, tick)
	topics := []common.Hash{
		chainabi.TopicUniswapV3Swap,
		common.BytesToHash(sender.Bytes()),
		common.BytesToHash(recipient.Bytes()),
	}

	ev, err := chainabi.DecodeUniswapV3Swap(topics, data)
	if err != nil {
		t.Fatalf("DecodeUniswapV3Swap: %v", err)
	}
	if ev.Sender != sender {
		t.Errorf("Sender = %s, want %s", ev.Sender, sender)
	}
	if ev.Recipient != recipient {
		t.Errorf("Recipient = %s, want %s", ev.Recipient, recipient)
	}
}

// ── Timeboost Deposit ───────────────────────────────────────────────────────

func TestDecodeTimeboostDeposit(t *testing.T) {
	t.Parallel()

	account := common.HexToAddress("0xabababababababababababababababababababababab"[:42])
	amount := new(big.Int).SetUint64(500_000_000_000_000_000) // 0.5 ETH

	data := common.LeftPadBytes(amount.Bytes(), 32)
	topics := []common.Hash{
		chainabi.TopicTimeboostDeposit,
		common.BytesToHash(account.Bytes()),
	}

	tests := []struct {
		name    string
		topics  []common.Hash
		data    []byte
		wantErr bool
	}{
		{name: "valid deposit", topics: topics, data: data, wantErr: false},
		{name: "too few topics", topics: topics[:1], data: data, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ev, err := chainabi.DecodeTimeboostDeposit(tc.topics, tc.data)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ev.Account != account {
				t.Errorf("Account = %s, want %s", ev.Account, account)
			}
			if ev.Amount.Cmp(amount) != 0 {
				t.Errorf("Amount = %s, want %s", ev.Amount, amount)
			}
		})
	}
}

// ── Topic hash correctness ───────────────────────────────────────────────────

func TestTopicHashesNotZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		topic common.Hash
	}{
		{"UniswapV3Swap", chainabi.TopicUniswapV3Swap},
		{"AlgebraSwap", chainabi.TopicAlgebraSwap},
		{"AaveV3LiquidationCall", chainabi.TopicAaveV3LiquidationCall},
		{"ERC20Transfer", chainabi.TopicERC20Transfer},
		{"TimeboostAuctionResolved", chainabi.TopicTimeboostAuctionResolved},
		{"TimeboostDeposit", chainabi.TopicTimeboostDeposit},
	}

	zero := common.Hash{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.topic == zero {
				t.Errorf("%s topic hash is zero — ABI parse failed", tc.name)
			}
		})
	}
}

// ERC-20 Transfer topic0 is a well-known value verifiable from EIP-20.
// keccak256("Transfer(address,address,uint256)") = ddf252ad...
func TestERC20TransferTopic0KnownValue(t *testing.T) {
	t.Parallel()
	want := "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	got := strings.ToLower(hex.EncodeToString(chainabi.TopicERC20Transfer.Bytes()))
	if got != want {
		t.Errorf("ERC20 Transfer topic0 = %s, want %s", got, want)
	}
}

// Uniswap V3 Swap topic0 is verifiable from on-chain data.
// keccak256("Swap(address,address,int256,int256,uint160,uint128,int24)") = c42079f9...
func TestUniswapV3SwapTopic0KnownValue(t *testing.T) {
	t.Parallel()
	want := "c42079f94a6350d7e6235f29174924f928cc2ac818eb64fed8004e115fbcca67"
	got := strings.ToLower(hex.EncodeToString(chainabi.TopicUniswapV3Swap.Bytes()))
	if got != want {
		t.Errorf("UniswapV3 Swap topic0 = %s, want %s", got, want)
	}
}

// ── Fuzz: log data decoder never panics ─────────────────────────────────────

func FuzzDecodeERC20Transfer(f *testing.F) {
	// Seed with a valid 32-byte padded value.
	f.Add(common.LeftPadBytes(big.NewInt(1e18).Bytes(), 32))
	f.Add([]byte{})
	f.Add(make([]byte, 100))

	from := common.HexToAddress("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	to := common.HexToAddress("0xcafecafecafecafecafecafecafecafecafecafe")
	topics := []common.Hash{
		chainabi.TopicERC20Transfer,
		common.BytesToHash(from.Bytes()),
		common.BytesToHash(to.Bytes()),
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic regardless of input.
		_, _ = chainabi.DecodeERC20Transfer(topics, data)
	})
}

// ── helpers ──────────────────────────────────────────────────────────────────

// encodeSwapData packs the non-indexed fields of a Uniswap V3 Swap event.
// All fields are ABI-encoded as 32-byte words (Ethereum ABI spec §4.1).
func encodeSwapData(amount0, amount1, sqrtPrice, liquidity *big.Int, tick int32) []byte {
	padSigned := func(n *big.Int) []byte {
		b := make([]byte, 32)
		if n.Sign() < 0 {
			// Two's complement for negative values.
			twos := new(big.Int).Add(n, new(big.Int).Lsh(big.NewInt(1), 256))
			twos.FillBytes(b)
		} else {
			n.FillBytes(b)
		}
		return b
	}

	var data []byte
	data = append(data, padSigned(amount0)...)
	data = append(data, padSigned(amount1)...)
	data = append(data, padSigned(sqrtPrice)...)
	data = append(data, padSigned(liquidity)...)
	tickBig := big.NewInt(int64(tick))
	data = append(data, padSigned(tickBig)...)
	return data
}
