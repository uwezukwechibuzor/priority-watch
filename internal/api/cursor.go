package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// keysetCursor is the internal representation of a keyset pagination cursor.
// The opaque string exposed in API responses is base64(json(keysetCursor)).
type keysetCursor struct {
	BlockNumber uint64  `json:"b"`
	TxIndex     *uint32 `json:"t,omitempty"`
}

// encodeCursor encodes a block number (and optional tx index) into an opaque
// cursor string suitable for API responses.
func encodeCursor(blockNumber uint64, txIndex *uint32) string {
	c := keysetCursor{BlockNumber: blockNumber, TxIndex: txIndex}
	data, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(data)
}

// decodeCursor decodes an opaque cursor string back to its components.
// Returns an error if the cursor is malformed.
func decodeCursor(s string) (blockNumber uint64, txIndex *uint32, err error) {
	if s == "" {
		return 0, nil, fmt.Errorf("empty cursor")
	}
	data, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid cursor: %w", err)
	}
	var c keysetCursor
	if err := json.Unmarshal(data, &c); err != nil {
		return 0, nil, fmt.Errorf("invalid cursor: %w", err)
	}
	return c.BlockNumber, c.TxIndex, nil
}
