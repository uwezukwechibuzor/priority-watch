package ingest

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
)

// bigCmpOpt allows comparing *big.Int by value.
var bigCmpOpt = cmp.Comparer(func(a, b *big.Int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Cmp(b) == 0
})

func mustBig(s string) *hexutil.Big {
	n := new(big.Int)
	n.SetString(s, 10)
	b := (*hexutil.Big)(n)
	return b
}

func hexU64(n uint64) hexutil.Uint64  { return hexutil.Uint64(n) }
func hexAddr(s string) common.Address { return common.HexToAddress(s) }
func hexHash(s string) common.Hash    { return common.HexToHash(s) }

func makeMinimalBlock() *chain.RPCBlock {
	idx := hexutil.Uint64(0)
	return &chain.RPCBlock{
		RPCBlockHeader: chain.RPCBlockHeader{
			Number:        hexU64(1000),
			Hash:          hexHash("0xabcdef"),
			ParentHash:    hexHash("0x123456"),
			Timestamp:     hexU64(1_700_000_000),
			L1BlockNumber: hexU64(99),
			BaseFeePerGas: mustBig("1000000000"),
			GasUsed:       hexU64(21000),
			GasLimit:      hexU64(30_000_000),
			Size:          hexU64(512),
		},
		Transactions: []chain.RPCTransaction{
			{
				Hash:                 hexHash("0x111"),
				TransactionIndex:     &idx,
				From:                 hexAddr("0xaaa"),
				To:                   &[]common.Address{hexAddr("0xbbb")}[0],
				Value:                mustBig("1000000000000000000"),
				Gas:                  hexU64(21000),
				GasPrice:             mustBig("2000000000"),
				Type:                 hexU64(2),
				Nonce:                hexU64(42),
				Input:                hexutil.Bytes{0xde, 0xad, 0xbe, 0xef, 0x01},
				MaxFeePerGas:         mustBig("3000000000"),
				MaxPriorityFeePerGas: mustBig("500000000"),
			},
		},
	}
}

func makeMinimalReceipt(blockNum, txIdx uint64, txHash common.Hash) chain.RPCReceipt {
	return chain.RPCReceipt{
		TransactionHash:   txHash,
		BlockNumber:       hexutil.Uint64(blockNum),
		TransactionIndex:  hexutil.Uint64(txIdx),
		Status:            hexU64(1),
		GasUsed:           hexU64(21000),
		GasUsedForL1:      hexU64(500),
		EffectiveGasPrice: mustBig("1500000000"),
		CumulativeGasUsed: hexU64(21000),
		ContractAddress:   nil,
		Logs:              nil,
	}
}

func TestTransformBlock_BlockRow(t *testing.T) {
	t.Parallel()

	block := makeMinimalBlock()
	receipts := []chain.RPCReceipt{makeMinimalReceipt(1000, 0, hexHash("0x111"))}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	blk, _, _, _ := TransformBlock(block, receipts, nil, now)

	want := BlockRow{
		BlockNumber:    1000,
		BlockHash:      hexHash("0xabcdef").Hex(),
		ParentHash:     hexHash("0x123456").Hex(),
		BlockTimestamp: time.Unix(1_700_000_000, 0).UTC(),
		L1BlockNumber:  99,
		BaseFeePerGas:  big.NewInt(1_000_000_000),
		GasUsed:        21000,
		GasLimit:       30_000_000,
		TxCount:        1,
		Size:           512,
		IngestedAt:     now.UTC(),
		Version:        uint64(now.UnixMilli()),
	}

	if diff := cmp.Diff(want, blk, bigCmpOpt, cmpopts.IgnoreUnexported()); diff != "" {
		t.Errorf("BlockRow mismatch (-want +got):\n%s", diff)
	}
}

func TestTransformBlock_TxRow(t *testing.T) {
	t.Parallel()

	block := makeMinimalBlock()
	receipts := []chain.RPCReceipt{makeMinimalReceipt(1000, 0, hexHash("0x111"))}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	_, txs, _, _ := TransformBlock(block, receipts, nil, now)
	if len(txs) != 1 {
		t.Fatalf("expected 1 tx, got %d", len(txs))
	}
	tx := txs[0]

	if tx.InputSelector != "0xdeadbeef" {
		t.Errorf("InputSelector: want 0xdeadbeef, got %s", tx.InputSelector)
	}
	if tx.InputSize != 5 {
		t.Errorf("InputSize: want 5, got %d", tx.InputSize)
	}
	if tx.TxType != 2 {
		t.Errorf("TxType: want 2, got %d", tx.TxType)
	}
	if tx.GasLimit != 21000 {
		t.Errorf("GasLimit: want 21000, got %d", tx.GasLimit)
	}
	if tx.MaxFeePerGas == nil || tx.MaxFeePerGas.Cmp(big.NewInt(3_000_000_000)) != 0 {
		t.Errorf("MaxFeePerGas unexpected: %v", tx.MaxFeePerGas)
	}
	if tx.MaxPriorityFeePerGas == nil || tx.MaxPriorityFeePerGas.Cmp(big.NewInt(500_000_000)) != 0 {
		t.Errorf("MaxPriorityFeePerGas unexpected: %v", tx.MaxPriorityFeePerGas)
	}
}

func TestTransformBlock_ReceiptRow(t *testing.T) {
	t.Parallel()

	block := makeMinimalBlock()
	receipts := []chain.RPCReceipt{makeMinimalReceipt(1000, 0, hexHash("0x111"))}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	_, _, rcpts, _ := TransformBlock(block, receipts, nil, now)
	if len(rcpts) != 1 {
		t.Fatalf("expected 1 receipt, got %d", len(rcpts))
	}
	rc := rcpts[0]

	if rc.GasUsedForL1 != 500 {
		t.Errorf("GasUsedForL1: want 500, got %d", rc.GasUsedForL1)
	}
	if rc.LogCount != 0 {
		t.Errorf("LogCount: want 0, got %d", rc.LogCount)
	}
	if rc.ContractAddress != "" {
		t.Errorf("ContractAddress: want empty, got %s", rc.ContractAddress)
	}
}

func TestTransformBlock_LogRow(t *testing.T) {
	t.Parallel()

	block := makeMinimalBlock()
	receipts := []chain.RPCReceipt{makeMinimalReceipt(1000, 0, hexHash("0x111"))}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	logs := []chain.RPCLog{
		{
			Address:          hexAddr("0xpool"),
			Topics:           []common.Hash{hexHash("0xtopic0"), hexHash("0xtopic1")},
			Data:             []byte{0x01, 0x02},
			BlockNumber:      hexU64(1000),
			TransactionHash:  hexHash("0x111"),
			TransactionIndex: hexU64(0),
			LogIndex:         hexU64(0),
			Removed:          false,
		},
		{
			// removed logs must be filtered out
			Address:          hexAddr("0xpool"),
			Topics:           []common.Hash{hexHash("0xdeadbeef")},
			Data:             nil,
			BlockNumber:      hexU64(1000),
			TransactionHash:  hexHash("0x111"),
			TransactionIndex: hexU64(0),
			LogIndex:         hexU64(1),
			Removed:          true,
		},
	}

	_, _, _, evts := TransformBlock(block, receipts, logs, now)
	if len(evts) != 1 {
		t.Fatalf("expected 1 log (removed one filtered), got %d", len(evts))
	}
	ev := evts[0]

	if ev.Topic1 != hexHash("0xtopic1").Hex() {
		t.Errorf("Topic1: want %s, got %s", hexHash("0xtopic1").Hex(), ev.Topic1)
	}
	if ev.Topic2 != "" {
		t.Errorf("Topic2: want empty, got %s", ev.Topic2)
	}
	if ev.Data != "0x0102" {
		t.Errorf("Data: want 0x0102, got %s", ev.Data)
	}
}

func TestInputSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"empty calldata", nil, ""},
		{"3 bytes", []byte{0x01, 0x02, 0x03}, ""},
		{"exactly 4 bytes", []byte{0xde, 0xad, 0xbe, 0xef}, "0xdeadbeef"},
		{"5 bytes", []byte{0xde, 0xad, 0xbe, 0xef, 0x01}, "0xdeadbeef"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := inputSelector(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("inputSelector mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHexEncode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"nil", nil, "0x"},
		{"empty", []byte{}, "0x"},
		{"one byte", []byte{0xff}, "0xff"},
		{"two bytes", []byte{0xab, 0xcd}, "0xabcd"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := hexEncode(tc.input)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("hexEncode mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
