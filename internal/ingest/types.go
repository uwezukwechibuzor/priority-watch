package ingest

import (
	"math/big"
	"time"
)

// ChunkStatus represents the state of a backfill chunk in meta.ingest_cursors.
type ChunkStatus string

// Chunk lifecycle states stored in meta.ingest_cursors.
const (
	StatusPending ChunkStatus = "pending"
	StatusRunning ChunkStatus = "running"
	StatusDone    ChunkStatus = "done"
	StatusFailed  ChunkStatus = "failed"
)

// Chunk is a contiguous block range to be ingested as one unit.
type Chunk struct {
	From uint64
	To   uint64
}

// BlockRow matches raw.blocks column order exactly.
type BlockRow struct {
	BlockNumber    uint64
	BlockHash      string
	ParentHash     string
	BlockTimestamp time.Time
	L1BlockNumber  uint64
	BaseFeePerGas  *big.Int
	GasUsed        uint64
	GasLimit       uint64
	TxCount        uint32
	Size           uint32
	IngestedAt     time.Time
	Version        uint64
}

// TxRow matches raw.transactions column order exactly.
// See 0004_raw_transactions.sql for the canonical definition.
type TxRow struct {
	BlockNumber          uint64
	BlockTimestamp       time.Time
	TxIndex              uint32
	TxHash               string
	TxType               uint8
	FromAddress          string
	ToAddress            string // empty for contract creation
	Nonce                uint64
	Value                *big.Int
	GasLimit             uint64
	GasPrice             *big.Int
	MaxFeePerGas         *big.Int // nil for legacy/type-0 txs
	MaxPriorityFeePerGas *big.Int // nil for legacy/type-0 txs
	InputSelector        string   // first 4 bytes of calldata, "0x"+8 hex chars; "" if < 4 bytes
	InputSize            uint32
	IngestedAt           time.Time
	Version              uint64
}

// ReceiptRow matches raw.receipts column order exactly.
// See 0005_raw_receipts.sql for the canonical definition.
type ReceiptRow struct {
	BlockNumber       uint64
	BlockTimestamp    time.Time
	TxIndex           uint32
	TxHash            string
	Status            uint8
	GasUsed           uint64
	GasUsedForL1      uint64
	EffectiveGasPrice *big.Int
	CumulativeGasUsed uint64
	ContractAddress   string // empty for non-deployment txs
	LogCount          uint32
	IngestedAt        time.Time
	Version           uint64
}

// ReorgLogRow matches raw.reorg_log column order exactly.
type ReorgLogRow struct {
	DetectedAt time.Time
	ForkBlock  uint64
	OldHash    string
	NewHash    string
	Depth      uint32
}

// BalanceSnapshotRow matches raw.balance_snapshots column order exactly.
type BalanceSnapshotRow struct {
	Address        string
	BlockNumber    uint64
	BlockTimestamp time.Time
	BalanceWei     *big.Int
	IngestedAt     time.Time
	Version        uint64
}

// LogRow matches raw.logs column order exactly.
// See 0006_raw_logs.sql for the canonical definition.
// data is stored as "0x"-prefixed hex of the raw ABI-encoded non-indexed params.
type LogRow struct {
	BlockNumber    uint64
	BlockTimestamp time.Time
	TxIndex        uint32
	TxHash         string
	LogIndex       uint32
	Address        string
	Topic0         string
	Topic1         string // empty string if not present
	Topic2         string
	Topic3         string
	Data           string // "0x"-prefixed hex
	IngestedAt     time.Time
	Version        uint64
}
