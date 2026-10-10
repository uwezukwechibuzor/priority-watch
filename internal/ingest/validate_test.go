package ingest

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
)

func TestValidateBlock(t *testing.T) {
	t.Parallel()

	hash1 := common.HexToHash("0xaaaa")
	hash2 := common.HexToHash("0xbbbb")

	makeTx := func(h common.Hash) chain.RPCTransaction {
		return chain.RPCTransaction{Hash: h}
	}
	makeReceipt := func(txHash common.Hash, blockNum uint64) chain.RPCReceipt {
		return chain.RPCReceipt{
			TransactionHash: txHash,
			BlockNumber:     hexutil.Uint64(blockNum),
		}
	}

	tests := []struct {
		name     string
		block    chain.RPCBlock
		receipts []chain.RPCReceipt
		wantErr  bool
	}{
		{
			name: "empty block",
			block: chain.RPCBlock{
				Number:       1,
				Transactions: nil,
			},
			receipts: nil,
			wantErr:  false,
		},
		{
			name: "matching tx and receipt",
			block: chain.RPCBlock{
				Number:       100,
				Transactions: []chain.RPCTransaction{makeTx(hash1)},
			},
			receipts: []chain.RPCReceipt{makeReceipt(hash1, 100)},
			wantErr:  false,
		},
		{
			name: "receipt count mismatch",
			block: chain.RPCBlock{
				Number:       100,
				Transactions: []chain.RPCTransaction{makeTx(hash1)},
			},
			receipts: []chain.RPCReceipt{},
			wantErr:  true,
		},
		{
			name: "receipt tx hash mismatch",
			block: chain.RPCBlock{
				Number:       100,
				Transactions: []chain.RPCTransaction{makeTx(hash1)},
			},
			receipts: []chain.RPCReceipt{makeReceipt(hash2, 100)},
			wantErr:  true,
		},
		{
			name: "receipt block number mismatch",
			block: chain.RPCBlock{
				Number:       100,
				Transactions: []chain.RPCTransaction{makeTx(hash1)},
			},
			receipts: []chain.RPCReceipt{makeReceipt(hash1, 99)},
			wantErr:  true,
		},
		{
			name: "two txs both match",
			block: chain.RPCBlock{
				Number:       200,
				Transactions: []chain.RPCTransaction{makeTx(hash1), makeTx(hash2)},
			},
			receipts: []chain.RPCReceipt{makeReceipt(hash1, 200), makeReceipt(hash2, 200)},
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateBlock(&tc.block, tc.receipts)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateBlockHash(t *testing.T) {
	t.Parallel()

	blockHash := common.HexToHash("0xdeadbeef")
	otherHash := common.HexToHash("0xcafebabe")

	tests := []struct {
		name               string
		blockHash          common.Hash
		expectedParentHash common.Hash
		wantErr            bool
	}{
		{
			name:               "hashes match",
			blockHash:          blockHash,
			expectedParentHash: blockHash,
			wantErr:            false,
		},
		{
			name:               "hashes differ",
			blockHash:          blockHash,
			expectedParentHash: otherHash,
			wantErr:            true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			block := &chain.RPCBlock{
				Number: 1,
				Hash:   tc.blockHash,
			}
			err := ValidateBlockHash(block, tc.expectedParentHash)
			if tc.wantErr != (err != nil) {
				t.Errorf("wantErr=%v but got err=%v", tc.wantErr, err)
			}
			_ = errors.New("unused") // suppress unused import warning
		})
	}
}
