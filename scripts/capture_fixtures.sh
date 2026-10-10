#!/usr/bin/env bash
# capture_fixtures.sh — fetch ~50 real Arbitrum One blocks from the RPC and save them as JSON
# fixtures in testdata/fixtures/.
#
# Usage:
#   ARBITRUM_RPC_URL=https://arb1.arbitrum.io/rpc ./scripts/capture_fixtures.sh
#   ARBITRUM_RPC_URL=... BLOCK_START=300000000 COUNT=50 ./scripts/capture_fixtures.sh
#
# Requirements: curl, jq

set -euo pipefail

RPC_URL="${ARBITRUM_RPC_URL:?ARBITRUM_RPC_URL must be set}"
BLOCK_START="${BLOCK_START:-300000000}"
COUNT="${COUNT:-50}"
OUT_DIR="${OUTDIR:-testdata/fixtures}"

mkdir -p "$OUT_DIR/blocks" "$OUT_DIR/receipts" "$OUT_DIR/logs"

echo "Capturing $COUNT blocks starting at $BLOCK_START from $RPC_URL"

for i in $(seq 0 $((COUNT - 1))); do
  BLOCK_NUM=$((BLOCK_START + i))
  HEX_NUM=$(printf "0x%x" "$BLOCK_NUM")

  # Fetch block with full transactions.
  BLOCK_FILE="$OUT_DIR/blocks/block_${BLOCK_NUM}.json"
  if [ ! -f "$BLOCK_FILE" ]; then
    curl -s -X POST "$RPC_URL" \
      -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"eth_getBlockByNumber\",\"params\":[\"$HEX_NUM\",true]}" \
      | jq '.result' > "$BLOCK_FILE"
    echo "  block $BLOCK_NUM saved"
  else
    echo "  block $BLOCK_NUM already exists, skipping"
  fi

  # Fetch receipts.
  RECEIPT_FILE="$OUT_DIR/receipts/receipts_${BLOCK_NUM}.json"
  if [ ! -f "$RECEIPT_FILE" ]; then
    BLOCK_HASH=$(jq -r '.hash' "$BLOCK_FILE")
    curl -s -X POST "$RPC_URL" \
      -H "Content-Type: application/json" \
      -d "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"eth_getBlockReceipts\",\"params\":[\"$BLOCK_HASH\"]}" \
      | jq '.result // []' > "$RECEIPT_FILE"
    echo "  receipts $BLOCK_NUM saved"
  fi

done

echo "Done. Fixtures in $OUT_DIR"
