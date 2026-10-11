package sqd

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
)

// SQDBatch is one NDJSON batch line: a block header plus its transactions,
// logs and receipts.
type SQDBatch struct {
	Header       SQDBlockHeader   `json:"header"`
	Transactions []SQDTransaction `json:"transactions"`
	Logs         []SQDLog         `json:"logs"`
	Receipts     []SQDReceipt     `json:"receipts"`
}

// SQDBlockHeader is the SQD block header object.
type SQDBlockHeader struct {
	Number        uint64 `json:"number"`
	Hash          string `json:"hash"`
	ParentHash    string `json:"parentHash"`
	Timestamp     uint64 `json:"timestamp"`
	GasUsed       string `json:"gasUsed"`       // hex string
	GasLimit      string `json:"gasLimit"`      // hex string
	BaseFeePerGas string `json:"baseFeePerGas"` // hex string
	L1BlockNumber uint64 `json:"l1BlockNumber"`
	Size          uint64 `json:"size"`
}

// SQDTransaction is the SQD transaction object.
type SQDTransaction struct {
	Hash                 string `json:"hash"`
	TransactionIndex     uint64 `json:"transactionIndex"`
	From                 string `json:"from"`
	To                   string `json:"to,omitempty"`
	Value                string `json:"value"`    // hex
	Gas                  string `json:"gas"`      // hex
	GasPrice             string `json:"gasPrice"` // hex
	MaxFeePerGas         string `json:"maxFeePerGas,omitempty"`
	MaxPriorityFeePerGas string `json:"maxPriorityFeePerGas,omitempty"`
	Type                 uint64 `json:"type"`
	Nonce                uint64 `json:"nonce"`
	Input                string `json:"input"` // hex bytes
	BlockNumber          uint64 `json:"blockNumber"`
}

// SQDLog is the SQD log object.
type SQDLog struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"` // hex
	LogIndex         uint64   `json:"logIndex"`
	TransactionIndex uint64   `json:"transactionIndex"`
	TransactionHash  string   `json:"transactionHash"`
	BlockNumber      uint64   `json:"blockNumber"`
}

// SQDReceipt is the SQD receipt object.
type SQDReceipt struct {
	TransactionHash   string `json:"transactionHash"`
	TransactionIndex  uint64 `json:"transactionIndex"`
	BlockNumber       uint64 `json:"blockNumber"`
	Status            uint64 `json:"status"`
	GasUsed           string `json:"gasUsed"`
	GasUsedForL1      string `json:"gasUsedForL1,omitempty"` // hex, Arbitrum-specific
	EffectiveGasPrice string `json:"effectiveGasPrice"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	ContractAddress   string `json:"contractAddress,omitempty"`
}

// SQDForkResponse is the 409 reorg body.
type SQDForkResponse struct {
	Error          string             `json:"error"`
	PreviousBlocks []SQDPreviousBlock `json:"previousBlocks"`
}

// SQDPreviousBlock is one entry of a fork response's previousBlocks list.
type SQDPreviousBlock struct {
	Number uint64 `json:"number"`
	Hash   string `json:"hash"`
}

// SQDStatusLine is a non-batch NDJSON status line (e.g. finalizedHeadNumber).
type SQDStatusLine struct {
	Status string `json:"status"`
	Value  uint64 `json:"value"`
}

// ── conversions to chain.RPC* wire types ──────────────────────────────────────

// hexBig parses a hex string ("0x...") into a *hexutil.Big. Empty input yields nil.
func hexBig(s string) (*hexutil.Big, error) {
	if s == "" {
		return nil, nil
	}
	s = ensure0x(s)
	v, err := hexutil.DecodeBig(s)
	if err != nil {
		return nil, fmt.Errorf("sqd: decode hex big %q: %w", s, err)
	}
	hb := (*hexutil.Big)(v)
	return hb, nil
}

// hexUint parses a hex string into a uint64. Empty input yields 0.
func hexUint(s string) (uint64, error) {
	if s == "" {
		return 0, nil
	}
	v, err := hexutil.DecodeUint64(ensure0x(s))
	if err != nil {
		return 0, fmt.Errorf("sqd: decode hex uint %q: %w", s, err)
	}
	return v, nil
}

// ensure0x prepends "0x" if the string lacks it.
func ensure0x(s string) string {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return s
	}
	return "0x" + s
}

// toRPCBlock converts an SQD header plus its transactions into a chain.RPCBlock.
func toRPCBlock(header SQDBlockHeader, txs []SQDTransaction) (*chain.RPCBlock, error) {
	gasUsed, err := hexUint(header.GasUsed)
	if err != nil {
		return nil, err
	}
	gasLimit, err := hexUint(header.GasLimit)
	if err != nil {
		return nil, err
	}
	baseFee, err := hexBig(header.BaseFeePerGas)
	if err != nil {
		return nil, err
	}

	rpcTxs := make([]chain.RPCTransaction, 0, len(txs))
	for i := range txs {
		rt, terr := toRPCTransaction(txs[i])
		if terr != nil {
			return nil, terr
		}
		rpcTxs = append(rpcTxs, rt)
	}

	return &chain.RPCBlock{
		Number:        hexutil.Uint64(header.Number),
		Hash:          common.HexToHash(header.Hash),
		ParentHash:    common.HexToHash(header.ParentHash),
		Timestamp:     hexutil.Uint64(header.Timestamp),
		L1BlockNumber: hexutil.Uint64(header.L1BlockNumber),
		BaseFeePerGas: baseFee,
		GasUsed:       hexutil.Uint64(gasUsed),
		GasLimit:      hexutil.Uint64(gasLimit),
		Size:          hexutil.Uint64(header.Size),
		Transactions:  rpcTxs,
	}, nil
}

// toRPCTransaction converts an SQD transaction into a chain.RPCTransaction.
func toRPCTransaction(tx SQDTransaction) (chain.RPCTransaction, error) {
	gas, err := hexUint(tx.Gas)
	if err != nil {
		return chain.RPCTransaction{}, err
	}
	value, err := hexBig(tx.Value)
	if err != nil {
		return chain.RPCTransaction{}, err
	}
	gasPrice, err := hexBig(tx.GasPrice)
	if err != nil {
		return chain.RPCTransaction{}, err
	}
	maxFee, err := hexBig(tx.MaxFeePerGas)
	if err != nil {
		return chain.RPCTransaction{}, err
	}
	maxTip, err := hexBig(tx.MaxPriorityFeePerGas)
	if err != nil {
		return chain.RPCTransaction{}, err
	}
	input, err := hexBytes(tx.Input)
	if err != nil {
		return chain.RPCTransaction{}, err
	}

	var to *common.Address
	if tx.To != "" {
		a := common.HexToAddress(tx.To)
		to = &a
	}
	idx := hexutil.Uint64(tx.TransactionIndex)

	return chain.RPCTransaction{
		Hash:                 common.HexToHash(tx.Hash),
		TransactionIndex:     &idx,
		From:                 common.HexToAddress(tx.From),
		To:                   to,
		Value:                value,
		Gas:                  hexutil.Uint64(gas),
		GasPrice:             gasPrice,
		MaxFeePerGas:         maxFee,
		MaxPriorityFeePerGas: maxTip,
		Type:                 hexutil.Uint64(tx.Type),
		Nonce:                hexutil.Uint64(tx.Nonce),
		Input:                input,
	}, nil
}

// toRPCReceipt converts an SQD receipt into a chain.RPCReceipt.
func toRPCReceipt(r SQDReceipt, blockHash string, blockTimestamp uint64) (chain.RPCReceipt, error) {
	_ = blockTimestamp // timestamp is sourced from the block header, not receipts

	gasUsed, err := hexUint(r.GasUsed)
	if err != nil {
		return chain.RPCReceipt{}, err
	}
	gasUsedL1, err := hexUint(r.GasUsedForL1)
	if err != nil {
		return chain.RPCReceipt{}, err
	}
	cumulative, err := hexUint(r.CumulativeGasUsed)
	if err != nil {
		return chain.RPCReceipt{}, err
	}
	effGas, err := hexBig(r.EffectiveGasPrice)
	if err != nil {
		return chain.RPCReceipt{}, err
	}

	var contract *common.Address
	if r.ContractAddress != "" {
		a := common.HexToAddress(r.ContractAddress)
		contract = &a
	}

	return chain.RPCReceipt{
		TransactionHash:   common.HexToHash(r.TransactionHash),
		BlockHash:         common.HexToHash(blockHash),
		BlockNumber:       hexutil.Uint64(r.BlockNumber),
		TransactionIndex:  hexutil.Uint64(r.TransactionIndex),
		ContractAddress:   contract,
		Status:            hexutil.Uint64(r.Status),
		GasUsed:           hexutil.Uint64(gasUsed),
		CumulativeGasUsed: hexutil.Uint64(cumulative),
		EffectiveGasPrice: effGas,
		GasUsedForL1:      hexutil.Uint64(gasUsedL1),
	}, nil
}

// toRPCLog converts an SQD log into a chain.RPCLog.
func toRPCLog(l SQDLog) (chain.RPCLog, error) {
	data, err := hexBytes(l.Data)
	if err != nil {
		return chain.RPCLog{}, err
	}
	topics := make([]common.Hash, 0, len(l.Topics))
	for _, t := range l.Topics {
		topics = append(topics, common.HexToHash(t))
	}
	return chain.RPCLog{
		Address:          common.HexToAddress(l.Address),
		Topics:           topics,
		Data:             data,
		BlockNumber:      hexutil.Uint64(l.BlockNumber),
		TransactionHash:  common.HexToHash(l.TransactionHash),
		TransactionIndex: hexutil.Uint64(l.TransactionIndex),
		LogIndex:         hexutil.Uint64(l.LogIndex),
	}, nil
}

// hexBytes decodes a hex string into bytes. Empty or "0x" yields an empty slice.
func hexBytes(s string) (hexutil.Bytes, error) {
	if s == "" || s == "0x" {
		return hexutil.Bytes{}, nil
	}
	b, err := hexutil.Decode(ensure0x(s))
	if err != nil {
		return nil, fmt.Errorf("sqd: decode hex bytes: %w", err)
	}
	return b, nil
}
