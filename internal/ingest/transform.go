package ingest

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
)

// TransformBlock converts a raw RPC block into DB row structs for all four
// raw.* tables.  The receipts slice must have exactly len(block.Transactions)
// elements in the same order; validated by ValidateBlock before calling here.
// logs contains only the events from pool/contract addresses of interest
// (pre-filtered by the caller via the log filter config).
func TransformBlock(
	block *chain.RPCBlock,
	receipts []chain.RPCReceipt,
	logs []chain.RPCLog,
	now time.Time,
) (blk BlockRow, txs []TxRow, rcpts []ReceiptRow, evts []LogRow) {
	ver := uint64(now.UnixMilli())
	ts := time.Unix(int64(block.Timestamp), 0).UTC()

	blk = BlockRow{
		BlockNumber:    uint64(block.Number),
		BlockHash:      block.Hash.Hex(),
		ParentHash:     block.ParentHash.Hex(),
		BlockTimestamp: ts,
		L1BlockNumber:  uint64(block.L1BlockNumber),
		BaseFeePerGas:  hexBigOrZero(block.BaseFeePerGas),
		GasUsed:        uint64(block.GasUsed),
		GasLimit:       uint64(block.GasLimit),
		TxCount:        uint32(len(block.Transactions)),
		Size:           uint32(block.Size),
		IngestedAt:     now.UTC(),
		Version:        ver,
	}

	txs = make([]TxRow, 0, len(block.Transactions))
	rcpts = make([]ReceiptRow, 0, len(receipts))

	for i, tx := range block.Transactions {
		idx := uint32(i)
		if tx.TransactionIndex != nil {
			idx = uint32(*tx.TransactionIndex)
		}
		txs = append(txs, transformTx(block, &tx, idx, ts, now, ver))
		if i < len(receipts) {
			rcpts = append(rcpts, transformReceipt(block, &receipts[i], idx, ts, now, ver))
		}
	}

	evts = make([]LogRow, 0, len(logs))
	for _, l := range logs {
		if l.Removed {
			continue
		}
		evts = append(evts, transformLog(block, &l, ts, now, ver))
	}

	return
}

func transformTx(
	block *chain.RPCBlock,
	tx *chain.RPCTransaction,
	txIndex uint32,
	ts, now time.Time,
	ver uint64,
) TxRow {
	toAddr := ""
	if tx.To != nil {
		toAddr = strings.ToLower(tx.To.Hex())
	}

	var maxFee, maxTip *big.Int
	if tx.MaxFeePerGas != nil {
		maxFee = new(big.Int).Set(tx.MaxFeePerGas.ToInt())
	}
	if tx.MaxPriorityFeePerGas != nil {
		maxTip = new(big.Int).Set(tx.MaxPriorityFeePerGas.ToInt())
	}

	return TxRow{
		BlockNumber:          uint64(block.Number),
		BlockTimestamp:       ts,
		TxIndex:              txIndex,
		TxHash:               tx.Hash.Hex(),
		TxType:               uint8(tx.Type),
		FromAddress:          strings.ToLower(tx.From.Hex()),
		ToAddress:            toAddr,
		Nonce:                uint64(tx.Nonce),
		Value:                hexBigOrZero(tx.Value),
		GasLimit:             uint64(tx.Gas),
		GasPrice:             hexBigOrZero(tx.GasPrice),
		MaxFeePerGas:         maxFee,
		MaxPriorityFeePerGas: maxTip,
		InputSelector:        inputSelector(tx.Input),
		InputSize:            uint32(len(tx.Input)),
		IngestedAt:           now.UTC(),
		Version:              ver,
	}
}

func transformReceipt(
	block *chain.RPCBlock,
	r *chain.RPCReceipt,
	txIndex uint32,
	ts, now time.Time,
	ver uint64,
) ReceiptRow {
	contractAddr := ""
	if r.ContractAddress != nil && *r.ContractAddress != (common.Address{}) {
		contractAddr = strings.ToLower(r.ContractAddress.Hex())
	}

	return ReceiptRow{
		BlockNumber:       uint64(block.Number),
		BlockTimestamp:    ts,
		TxIndex:           txIndex,
		TxHash:            r.TransactionHash.Hex(),
		Status:            uint8(r.Status),
		GasUsed:           uint64(r.GasUsed),
		GasUsedForL1:      uint64(r.GasUsedForL1),
		EffectiveGasPrice: hexBigOrZero(r.EffectiveGasPrice),
		CumulativeGasUsed: uint64(r.CumulativeGasUsed),
		ContractAddress:   contractAddr,
		LogCount:          uint32(len(r.Logs)),
		IngestedAt:        now.UTC(),
		Version:           ver,
	}
}

func transformLog(block *chain.RPCBlock, l *chain.RPCLog, ts, now time.Time, ver uint64) LogRow {
	return LogRow{
		BlockNumber:    uint64(block.Number),
		BlockTimestamp: ts,
		TxIndex:        uint32(l.TransactionIndex),
		TxHash:         l.TransactionHash.Hex(),
		LogIndex:       uint32(l.LogIndex),
		Address:        strings.ToLower(l.Address.Hex()),
		Topic0:         topicOrEmpty(l.Topics, 0),
		Topic1:         topicOrEmpty(l.Topics, 1),
		Topic2:         topicOrEmpty(l.Topics, 2),
		Topic3:         topicOrEmpty(l.Topics, 3),
		Data:           hexEncode(l.Data),
		IngestedAt:     now.UTC(),
		Version:        ver,
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// hexBigOrZero safely converts a *hexutil.Big (which may be nil) to *big.Int.
// A nil pointer would escape the nil check when boxed into an interface, so
// this function takes the concrete type directly.
func hexBigOrZero(h *hexutil.Big) *big.Int {
	if h == nil {
		return new(big.Int)
	}
	v := h.ToInt()
	if v == nil {
		return new(big.Int)
	}
	return new(big.Int).Set(v)
}

func topicOrEmpty(topics []common.Hash, i int) string {
	if i >= len(topics) {
		return ""
	}
	return topics[i].Hex()
}

func hexEncode(b []byte) string {
	if len(b) == 0 {
		return "0x"
	}
	return "0x" + hex.EncodeToString(b)
}

// inputSelector extracts the first 4 bytes of calldata as "0x"+8-hex-char string.
// Returns "" for calldata shorter than 4 bytes.
func inputSelector(input []byte) string {
	if len(input) < 4 {
		return ""
	}
	return fmt.Sprintf("0x%s", hex.EncodeToString(input[:4]))
}
