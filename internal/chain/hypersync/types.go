package hypersync

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/uwezukwechibuzor/priority-watch/internal/chain"
)

// HyperSyncQuery is the JSON body posted to the /query endpoint.
type HyperSyncQuery struct {
	FromBlock        uint64           `json:"from_block"`
	ToBlock          *uint64          `json:"to_block,omitempty"`
	Logs             []HSLogFilter    `json:"logs,omitempty"`
	Transactions     []HSEmptyFilter  `json:"transactions,omitempty"`
	IncludeAllBlocks bool             `json:"include_all_blocks"`
	FieldSelection   HSFieldSelection `json:"field_selection"`
}

// HSLogFilter filters logs by address.
type HSLogFilter struct {
	Address []string `json:"address,omitempty"`
}

// HSEmptyFilter matches all transactions.
type HSEmptyFilter struct{}

// HSFieldSelection lists the fields to return for each entity.
type HSFieldSelection struct {
	Block       []string `json:"block"`
	Transaction []string `json:"transaction"`
	Receipt     []string `json:"receipt"`
	Log         []string `json:"log"`
}

// HyperSyncResponse is the /query response envelope.
type HyperSyncResponse struct {
	ArchiveHeight uint64       `json:"archive_height"`
	NextBlock     uint64       `json:"next_block"`
	TotalExecTime uint64       `json:"total_execution_time"`
	Data          []HSDataPage `json:"data"`
}

// HSDataPage is one page of block/tx/log/receipt data.
type HSDataPage struct {
	Blocks       []HSBlock       `json:"blocks"`
	Transactions []HSTransaction `json:"transactions"`
	Logs         []HSLog         `json:"logs"`
	Receipts     []HSReceipt     `json:"receipts"`
}

// HSBlock is the HyperSync block object.
type HSBlock struct {
	Number        uint64  `json:"number"`
	Hash          string  `json:"hash"`
	ParentHash    string  `json:"parent_hash"`
	Timestamp     uint64  `json:"timestamp"`
	GasUsed       uint64  `json:"gas_used"`
	GasLimit      uint64  `json:"gas_limit"`
	BaseFeePerGas *string `json:"base_fee_per_gas"` // decimal or hex string
	L1BlockNumber *uint64 `json:"l1_block_number"`
	Size          *uint64 `json:"size"`
}

// HSTransaction is the HyperSync transaction object.
type HSTransaction struct {
	Hash                 string  `json:"hash"`
	From                 string  `json:"from"`
	To                   *string `json:"to"`
	Value                *string `json:"value"`
	Gas                  uint64  `json:"gas"`
	GasPrice             *string `json:"gas_price"`
	MaxFeePerGas         *string `json:"max_fee_per_gas"`
	MaxPriorityFeePerGas *string `json:"max_priority_fee_per_gas"`
	TransactionType      uint8   `json:"transaction_type"`
	Nonce                uint64  `json:"nonce"`
	Input                string  `json:"input"` // hex
	TransactionIndex     uint64  `json:"transaction_index"`
	BlockNumber          uint64  `json:"block_number"`
}

// HSReceipt is the HyperSync receipt object.
type HSReceipt struct {
	TransactionHash   string  `json:"transaction_hash"`
	TransactionIndex  uint64  `json:"transaction_index"`
	BlockNumber       uint64  `json:"block_number"`
	Status            uint8   `json:"status"`
	GasUsed           uint64  `json:"gas_used"`
	GasUsedForL1      uint64  `json:"gas_used_for_l1"`
	EffectiveGasPrice *string `json:"effective_gas_price"`
	CumulativeGasUsed uint64  `json:"cumulative_gas_used"`
	ContractAddress   *string `json:"contract_address"`
}

// HSLog is the HyperSync log object (topics are split into topic0..topic3).
type HSLog struct {
	Address          string  `json:"address"`
	Topic0           *string `json:"topic0"`
	Topic1           *string `json:"topic1"`
	Topic2           *string `json:"topic2"`
	Topic3           *string `json:"topic3"`
	Data             string  `json:"data"`
	LogIndex         uint64  `json:"log_index"`
	TransactionIndex uint64  `json:"transaction_index"`
	TransactionHash  string  `json:"transaction_hash"`
	BlockNumber      uint64  `json:"block_number"`
}

// fieldSelection returns the fixed field selection for Priority Watch ingestion.
func fieldSelection() HSFieldSelection {
	return HSFieldSelection{
		Block: []string{
			"number", "hash", "parent_hash", "timestamp",
			"gas_used", "gas_limit", "base_fee_per_gas", "l1_block_number", "size",
		},
		Transaction: []string{
			"hash", "from", "to", "value", "gas", "gas_price",
			"max_fee_per_gas", "max_priority_fee_per_gas", "transaction_type",
			"nonce", "input", "transaction_index", "block_number",
		},
		Receipt: []string{
			"transaction_hash", "transaction_index", "block_number", "status",
			"gas_used", "gas_used_for_l1", "effective_gas_price",
			"cumulative_gas_used", "contract_address",
		},
		Log: []string{
			"address", "topic0", "topic1", "topic2", "topic3", "data",
			"log_index", "transaction_index", "transaction_hash", "block_number",
		},
	}
}

// ── conversions ───────────────────────────────────────────────────────────────

// numBig parses a decimal or hex numeric string into a *hexutil.Big.
// A nil or empty input yields nil.
func numBig(s *string) (*hexutil.Big, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	v, ok := new(big.Int).SetString(strings.TrimPrefix(strings.TrimPrefix(*s, "0x"), "0X"), base(*s))
	if !ok {
		return nil, fmt.Errorf("hypersync: parse numeric %q", *s)
	}
	hb := (*hexutil.Big)(v)
	return hb, nil
}

func base(s string) int {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return 16
	}
	return 10
}

// hsHexBytes decodes a hex string into bytes. Empty or "0x" yields empty.
func hsHexBytes(s string) (hexutil.Bytes, error) {
	if s == "" || s == "0x" {
		return hexutil.Bytes{}, nil
	}
	if !strings.HasPrefix(s, "0x") {
		s = "0x" + s
	}
	b, err := hexutil.Decode(s)
	if err != nil {
		return nil, fmt.Errorf("hypersync: decode hex bytes: %w", err)
	}
	return b, nil
}

// toRPCBlock converts an HSBlock plus its transactions into a chain.RPCBlock.
func toRPCBlock(b HSBlock, txs []HSTransaction) (*chain.RPCBlock, error) {
	baseFee, err := numBig(b.BaseFeePerGas)
	if err != nil {
		return nil, err
	}
	var l1 uint64
	if b.L1BlockNumber != nil {
		l1 = *b.L1BlockNumber
	}
	var size uint64
	if b.Size != nil {
		size = *b.Size
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
		RPCBlockHeader: chain.RPCBlockHeader{
			Number:        hexutil.Uint64(b.Number),
			Hash:          common.HexToHash(b.Hash),
			ParentHash:    common.HexToHash(b.ParentHash),
			Timestamp:     hexutil.Uint64(b.Timestamp),
			L1BlockNumber: hexutil.Uint64(l1),
			BaseFeePerGas: baseFee,
			GasUsed:       hexutil.Uint64(b.GasUsed),
			GasLimit:      hexutil.Uint64(b.GasLimit),
			Size:          hexutil.Uint64(size),
		},
		Transactions: rpcTxs,
	}, nil
}

// toRPCTransaction converts an HSTransaction into a chain.RPCTransaction.
func toRPCTransaction(tx HSTransaction) (chain.RPCTransaction, error) {
	value, err := numBig(tx.Value)
	if err != nil {
		return chain.RPCTransaction{}, err
	}
	gasPrice, err := numBig(tx.GasPrice)
	if err != nil {
		return chain.RPCTransaction{}, err
	}
	maxFee, err := numBig(tx.MaxFeePerGas)
	if err != nil {
		return chain.RPCTransaction{}, err
	}
	maxTip, err := numBig(tx.MaxPriorityFeePerGas)
	if err != nil {
		return chain.RPCTransaction{}, err
	}
	input, err := hsHexBytes(tx.Input)
	if err != nil {
		return chain.RPCTransaction{}, err
	}

	var to *common.Address
	if tx.To != nil && *tx.To != "" {
		a := common.HexToAddress(*tx.To)
		to = &a
	}
	idx := hexutil.Uint64(tx.TransactionIndex)

	return chain.RPCTransaction{
		Hash:                 common.HexToHash(tx.Hash),
		TransactionIndex:     &idx,
		From:                 common.HexToAddress(tx.From),
		To:                   to,
		Value:                value,
		Gas:                  hexutil.Uint64(tx.Gas),
		GasPrice:             gasPrice,
		MaxFeePerGas:         maxFee,
		MaxPriorityFeePerGas: maxTip,
		Type:                 hexutil.Uint64(tx.TransactionType),
		Nonce:                hexutil.Uint64(tx.Nonce),
		Input:                input,
	}, nil
}

// toRPCReceipt converts an HSReceipt into a chain.RPCReceipt.
func toRPCReceipt(r HSReceipt, blockHash string) (chain.RPCReceipt, error) {
	effGas, err := numBig(r.EffectiveGasPrice)
	if err != nil {
		return chain.RPCReceipt{}, err
	}
	var contract *common.Address
	if r.ContractAddress != nil && *r.ContractAddress != "" {
		a := common.HexToAddress(*r.ContractAddress)
		contract = &a
	}
	return chain.RPCReceipt{
		TransactionHash:   common.HexToHash(r.TransactionHash),
		BlockHash:         common.HexToHash(blockHash),
		BlockNumber:       hexutil.Uint64(r.BlockNumber),
		TransactionIndex:  hexutil.Uint64(r.TransactionIndex),
		ContractAddress:   contract,
		Status:            hexutil.Uint64(r.Status),
		GasUsed:           hexutil.Uint64(r.GasUsed),
		CumulativeGasUsed: hexutil.Uint64(r.CumulativeGasUsed),
		EffectiveGasPrice: effGas,
		GasUsedForL1:      hexutil.Uint64(r.GasUsedForL1),
	}, nil
}

// toRPCLog converts an HSLog into a chain.RPCLog.
func toRPCLog(l HSLog) (chain.RPCLog, error) {
	data, err := hsHexBytes(l.Data)
	if err != nil {
		return chain.RPCLog{}, err
	}
	var topics []common.Hash
	for _, t := range []*string{l.Topic0, l.Topic1, l.Topic2, l.Topic3} {
		if t == nil || *t == "" {
			break
		}
		topics = append(topics, common.HexToHash(*t))
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
