# Runbook: RPC Outage

## Detection
- `pw_follower_rpc_errors_total` rises in Grafana
- Block lag alert fires (head - latest_ingested > 20 blocks for 5 min)
- The follower logs: `level=error msg="rpc call failed"`

## Immediate response

### 1. Check the primary RPC
```bash
curl -s -X POST $ARBITRUM_RPC_URL \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'
```

### 2. Switch to fallback RPC
Update `ARBITRUM_RPC_URL` in `.env` to one of the fallbacks in
`ARBITRUM_RPC_FALLBACK_URLS`, then restart the follower:

```bash
docker compose restart pw-follower
```

### 3. Fill the gap
Once the primary RPC recovers or the fallback is active:
```bash
make backfill FROM=<last_good_block> TO=head
```

### 4. Verify
```bash
make smoke
```

## Prevention
- Use a paid RPC plan (free tiers cannot keep up at 4 blocks/s).
- Configure at least 2 fallback URLs in `ARBITRUM_RPC_FALLBACK_URLS`.
- Set `ARBITRUM_RPC_RPS` to respect your plan's rate limit.
