package ingest

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
)

// ValidateBlock checks that receipts correspond to the block's transactions.
// It returns an error if:
//   - len(receipts) != len(block.Transactions)
//   - any receipt's tx_hash doesn't match the corresponding transaction hash
//   - any receipt's block_number doesn't match the block number
func ValidateBlock(block *chain.RPCBlock, receipts []chain.RPCReceipt) error {
	if len(receipts) != len(block.Transactions) {
		return fmt.Errorf("block %d: receipt count %d != tx count %d",
			block.Number, len(receipts), len(block.Transactions))
	}
	for i, r := range receipts {
		tx := block.Transactions[i]
		if r.TransactionHash != tx.Hash {
			return fmt.Errorf("block %d tx[%d]: receipt hash %s != tx hash %s",
				block.Number, i, r.TransactionHash.Hex(), tx.Hash.Hex())
		}
		if uint64(r.BlockNumber) != uint64(block.Number) {
			return fmt.Errorf("block %d tx[%d]: receipt block number %d mismatch",
				block.Number, i, r.BlockNumber)
		}
	}
	return nil
}

// ValidateBlockHash checks that the block hash matches the parent hash of the
// next block, providing a lightweight chain-continuity assertion.
// Pass the block under validation and the next block's parent hash.
func ValidateBlockHash(block *chain.RPCBlock, expectedParentHash common.Hash) error {
	if block.Hash != expectedParentHash {
		return fmt.Errorf("block %d: hash %s does not match expected parent hash %s",
			block.Number, block.Hash.Hex(), expectedParentHash.Hex())
	}
	return nil
}
