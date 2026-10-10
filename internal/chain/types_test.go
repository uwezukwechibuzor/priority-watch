package chain_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
)

func TestDecodeBlock(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../testdata/chain/block_with_system_txs.json")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	var block chain.RPCBlock
	if err := json.Unmarshal(data, &block); err != nil {
		t.Fatalf("unmarshal block: %v", err)
	}

	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "block number decoded",
			fn: func(t *testing.T) {
				t.Helper()
				if got, want := uint64(block.Number), uint64(0x7f89a3c); got != want {
					t.Errorf("Number = 0x%x, want 0x%x", got, want)
				}
			},
		},
		{
			name: "l1BlockNumber present",
			fn: func(t *testing.T) {
				t.Helper()
				if got, want := uint64(block.L1BlockNumber), uint64(0x14a2d0); got != want {
					t.Errorf("L1BlockNumber = 0x%x, want 0x%x", got, want)
				}
			},
		},
		{
			name: "three transactions decoded",
			fn: func(t *testing.T) {
				t.Helper()
				if diff := cmp.Diff(3, len(block.Transactions)); diff != "" {
					t.Errorf("tx count mismatch (-want +got):\n%s", diff)
				}
			},
		},
		{
			name: "system tx type 0x64 decoded",
			fn: func(t *testing.T) {
				t.Helper()
				if got := uint64(block.Transactions[0].Type); got != 0x64 {
					t.Errorf("tx[0].Type = 0x%x, want 0x64", got)
				}
			},
		},
		{
			name: "system tx IsSystemTx true",
			fn: func(t *testing.T) {
				t.Helper()
				if !block.Transactions[0].IsSystemTx() {
					t.Error("tx[0].IsSystemTx() = false, want true")
				}
			},
		},
		{
			name: "EIP-1559 tx IsSystemTx false",
			fn: func(t *testing.T) {
				t.Helper()
				if block.Transactions[1].IsSystemTx() {
					t.Error("tx[1].IsSystemTx() = true, want false")
				}
			},
		},
		{
			name: "legacy tx IsSystemTx false",
			fn: func(t *testing.T) {
				t.Helper()
				if block.Transactions[2].IsSystemTx() {
					t.Error("tx[2].IsSystemTx() = true, want false")
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.fn(t)
		})
	}
}

func TestDecodeReceipt(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../testdata/chain/receipt_arbitrum.json")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	var receipt chain.RPCReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatalf("unmarshal receipt: %v", err)
	}

	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "status success",
			fn: func(t *testing.T) {
				t.Helper()
				if got, want := uint64(receipt.Status), uint64(1); got != want {
					t.Errorf("Status = %d, want %d", got, want)
				}
			},
		},
		{
			name: "gasUsedForL1 present",
			fn: func(t *testing.T) {
				t.Helper()
				if got, want := uint64(receipt.GasUsedForL1), uint64(0x1234); got != want {
					t.Errorf("GasUsedForL1 = 0x%x, want 0x1234", got)
				}
			},
		},
		{
			name: "l1BlockNumber present",
			fn: func(t *testing.T) {
				t.Helper()
				if got, want := uint64(receipt.L1BlockNumber), uint64(0x14a2d0); got != want {
					t.Errorf("L1BlockNumber = 0x%x, want 0x14a2d0", got)
				}
			},
		},
		{
			name: "contractAddress nil for non-deployment",
			fn: func(t *testing.T) {
				t.Helper()
				if receipt.ContractAddress != nil {
					t.Errorf("ContractAddress = %v, want nil", receipt.ContractAddress)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.fn(t)
		})
	}
}

// buildEIP1559TxJSON constructs a minimal EIP-1559 transaction JSON string.
func buildEIP1559TxJSON(maxFeeHex, maxTipHex string) string {
	return fmt.Sprintf(`{
		"hash":"0x0000000000000000000000000000000000000000000000000000000000000001",
		"from":"0x0000000000000000000000000000000000000000",
		"gas":"0x1",
		"gasPrice":"0x0",
		"maxFeePerGas":"%s",
		"maxPriorityFeePerGas":"%s",
		"type":"0x2",
		"nonce":"0x0",
		"input":"0x"
	}`, maxFeeHex, maxTipHex)
}

func TestPriorityFeePerGas(t *testing.T) {
	t.Parallel()

	baseFee := big.NewInt(100_000_000) // 0x5f5e100

	tests := []struct {
		name     string
		txJSON   string
		wantWei  int64
	}{
		{
			name:    "tip bounded by maxPriorityFeePerGas when cap is generous",
			txJSON:  buildEIP1559TxJSON("0x6f05b59d3b20000", "0x5f5e100"),
			wantWei: 100_000_000,
		},
		{
			name:    "tip capped at maxFeePerGas minus baseFee",
			txJSON:  buildEIP1559TxJSON("0x5f5e101", "0x11e1a300"), // maxFee=baseFee+1, tip much higher
			wantWei: 1,
		},
		{
			name:    "tip zero when maxFee equals baseFee",
			txJSON:  buildEIP1559TxJSON("0x5f5e100", "0x5f5e100"), // maxFee == baseFee
			wantWei: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var tx chain.RPCTransaction
			if err := json.Unmarshal([]byte(tc.txJSON), &tx); err != nil {
				t.Fatalf("unmarshal tx: %v", err)
			}
			got := tx.PriorityFeePerGas(baseFee)
			want := big.NewInt(tc.wantWei)
			if got.Cmp(want) != 0 {
				t.Errorf("PriorityFeePerGas = %s, want %s", got, want)
			}
		})
	}
}

func TestPriorityFeeSystemTxZero(t *testing.T) {
	t.Parallel()

	baseFee := big.NewInt(100)
	txJSON := `{"hash":"0x0000000000000000000000000000000000000000000000000000000000000001",
		"from":"0x0000000000000000000000000000000000000000",
		"gas":"0x0","gasPrice":"0x0","type":"0x64","nonce":"0x0","input":"0x"}`

	var tx chain.RPCTransaction
	if err := json.Unmarshal([]byte(txJSON), &tx); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := tx.PriorityFeePerGas(baseFee); got.Sign() != 0 {
		t.Errorf("system tx priority fee = %s, want 0", got)
	}
}
