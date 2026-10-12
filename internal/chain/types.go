// Package chain provides an Arbitrum One JSON-RPC client and custom wire types.
//
// go-ethereum's types.Transaction cannot decode Arbitrum system transaction
// types 0x64–0x6a (ArbitrumDepositTx, ArbitrumUnsignedTx, ArbitrumInternalTx,
// ArbitrumLegacyTx, ArbitrumRetryTx, ArbitrumSubmitRetryableTx,
// ArbitrumDepositedRetryableTx), and go-ethereum's block/receipt types drop
// Arbitrum-specific fields such as l1BlockNumber and gasUsedForL1.
// The types below decode the raw JSON faithfully without field loss.
package chain

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Arbitrum system transaction type identifiers (not decoded by go-ethereum).
const (
	ArbitrumDepositTxType         = 0x64
	ArbitrumUnsignedTxType        = 0x65
	ArbitrumContractTxType        = 0x66
	ArbitrumRetryTxType           = 0x68
	ArbitrumSubmitRetryableTxType = 0x69
	ArbitrumInternalTxType        = 0x6a
)

// RPCBlockHeader is the block header returned by eth_getBlockByNumber/Hash when
// the second arg is false (tx hashes only). The transactions field is omitted
// to avoid unmarshalling string hashes into RPCTransaction objects.
type RPCBlockHeader struct {
	Number        hexutil.Uint64 `json:"number"`
	Hash          common.Hash    `json:"hash"`
	ParentHash    common.Hash    `json:"parentHash"`
	Timestamp     hexutil.Uint64 `json:"timestamp"`
	L1BlockNumber hexutil.Uint64 `json:"l1BlockNumber"`
	BaseFeePerGas *hexutil.Big   `json:"baseFeePerGas"`
	GasUsed       hexutil.Uint64 `json:"gasUsed"`
	GasLimit      hexutil.Uint64 `json:"gasLimit"`
	Size          hexutil.Uint64 `json:"size"`
}

// RPCBlockWithHashes is returned by eth_getBlockByHash/Number with false — tx
// hashes only. Used by the BlockReceipts fallback path.
type RPCBlockWithHashes struct {
	RPCBlockHeader
	Transactions []common.Hash `json:"transactions"`
}

// RPCBlock is the raw JSON-RPC response for eth_getBlockByHash /
// eth_getBlockByNumber with full transaction objects (second arg = true).
// Arbitrum-specific: l1BlockNumber is present on all blocks.
type RPCBlock struct {
	RPCBlockHeader
	Transactions []RPCTransaction `json:"transactions"`
}

// RPCTransaction is the raw JSON-RPC transaction object.
// Fields optional on Arbitrum system transactions are pointers; absent fields
// decode to nil without error, which is safe because system txs are not
// subjected to PGA ordering and are excluded from tip analytics.
type RPCTransaction struct {
	Hash                 common.Hash     `json:"hash"`
	BlockHash            *common.Hash    `json:"blockHash"`
	BlockNumber          *hexutil.Uint64 `json:"blockNumber"`
	TransactionIndex     *hexutil.Uint64 `json:"transactionIndex"`
	From                 common.Address  `json:"from"`
	To                   *common.Address `json:"to"`
	Value                *hexutil.Big    `json:"value"`
	Gas                  hexutil.Uint64  `json:"gas"`
	GasPrice             *hexutil.Big    `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas"`
	Type                 hexutil.Uint64  `json:"type"`
	Nonce                hexutil.Uint64  `json:"nonce"`
	Input                hexutil.Bytes   `json:"input"`
}

// IsSystemTx returns true for Arbitrum internal transaction types that are
// not user-submitted and carry no priority fee.
func (tx *RPCTransaction) IsSystemTx() bool {
	t := uint8(tx.Type) //nolint:gosec
	return t >= ArbitrumDepositTxType && t <= ArbitrumInternalTxType
}

// PriorityFeePerGas computes min(maxPriorityFeePerGas, maxFeePerGas−baseFee).
// Returns zero for legacy / type-0 transactions and for system transactions.
// baseFee must not be nil.
func (tx *RPCTransaction) PriorityFeePerGas(baseFee *big.Int) *big.Int {
	zero := new(big.Int)
	if tx.IsSystemTx() || tx.MaxFeePerGas == nil || tx.MaxPriorityFeePerGas == nil {
		// Legacy tx: effective tip is gasPrice − baseFee (but floored at 0).
		if tx.GasPrice != nil {
			tip := new(big.Int).Sub(tx.GasPrice.ToInt(), baseFee)
			if tip.Sign() < 0 {
				return zero
			}
			return tip
		}
		return zero
	}
	// EIP-1559: tip = min(maxPriorityFeePerGas, maxFeePerGas − baseFee)
	cap := new(big.Int).Sub(tx.MaxFeePerGas.ToInt(), baseFee)
	tip := tx.MaxPriorityFeePerGas.ToInt()
	if cap.Cmp(tip) < 0 {
		return new(big.Int).Set(cap)
	}
	return new(big.Int).Set(tip)
}

// RPCReceipt is the raw JSON-RPC receipt object.
// Arbitrum-specific: gasUsedForL1, l1BlockNumber.
type RPCReceipt struct {
	TransactionHash   common.Hash     `json:"transactionHash"`
	BlockHash         common.Hash     `json:"blockHash"`
	BlockNumber       hexutil.Uint64  `json:"blockNumber"`
	TransactionIndex  hexutil.Uint64  `json:"transactionIndex"`
	From              common.Address  `json:"from"`
	To                *common.Address `json:"to"`
	ContractAddress   *common.Address `json:"contractAddress"`
	Status            hexutil.Uint64  `json:"status"`
	GasUsed           hexutil.Uint64  `json:"gasUsed"`
	CumulativeGasUsed hexutil.Uint64  `json:"cumulativeGasUsed"`
	EffectiveGasPrice *hexutil.Big    `json:"effectiveGasPrice"`
	GasUsedForL1      hexutil.Uint64  `json:"gasUsedForL1"`
	L1BlockNumber     hexutil.Uint64  `json:"l1BlockNumber"`
	Logs              []RPCLog        `json:"logs"`
}

// RPCLog is the raw JSON-RPC log object returned in receipts and eth_getLogs.
type RPCLog struct {
	Address          common.Address `json:"address"`
	Topics           []common.Hash  `json:"topics"`
	Data             hexutil.Bytes  `json:"data"`
	BlockNumber      hexutil.Uint64 `json:"blockNumber"`
	TransactionHash  common.Hash    `json:"transactionHash"`
	TransactionIndex hexutil.Uint64 `json:"transactionIndex"`
	BlockHash        common.Hash    `json:"blockHash"`
	LogIndex         hexutil.Uint64 `json:"logIndex"`
	Removed          bool           `json:"removed"`
}

// LogFilter is the argument for eth_getLogs.
type LogFilter struct {
	FromBlock string        `json:"fromBlock,omitempty"`
	ToBlock   string        `json:"toBlock,omitempty"`
	Address   []string      `json:"address,omitempty"`
	Topics    []interface{} `json:"topics,omitempty"`
}

// MarshalJSON implements json.Marshaler for LogFilter.
func (f LogFilter) MarshalJSON() ([]byte, error) {
	type alias LogFilter
	return json.Marshal(alias(f))
}
