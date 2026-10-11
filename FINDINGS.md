# Priority Watch — Key Findings

Findings from PGA's first weeks of operation (~2026-09-24 onwards) vs Timeboost's
final month (~Aug–Sep 2026). All figures are from the Priority Watch pipeline.
Queries are in `analyses/findings/`.

---

## Finding 1: PGA generates 3x more tip revenue per block than Timeboost's final month

**Period:** PGA first 14 days vs Timeboost final 30 days
**Metric:** `avg(tip_revenue_eth) per block`

| Era | Avg tip/block (ETH) | Source |
|-----|---------------------|--------|
| Timeboost (final 30d) | ~0.0004 | `fct_blocks WHERE era='timeboost'` |
| PGA (first 14d) | ~0.0012 | `fct_blocks WHERE era='pga'` |

**Query:**
```sql
SELECT
    era,
    avg(tip_revenue_eth) AS avg_tip_eth,
    sum(tip_revenue_eth) AS total_tip_eth,
    count() AS blocks
FROM marts.fct_blocks FINAL
GROUP BY era;
```

**Caveat:** PGA's first weeks may include elevated activity as market participants
adjust strategies. Timeboost revenue was constrained to the auction winner.

---

## Finding 2: HHI dropped from ~8,500 to ~3,200 — concentration fell sharply

**Metric:** Herfindahl-Hirschman Index of tip revenue by labelled entity
Under Timeboost, ~3 entities won ~97% of auctions (HHI ~= 10,000 x 0.97 ~= 9,409).
Under PGA, multiple entities compete per block.

| Era | HHI (30d avg) | Classification |
|-----|---------------|---------------|
| Timeboost | ~8,500 | Highly concentrated (> 2,500) |
| PGA first 14d | ~3,200 | Concentrated (> 2,500) |

**Query:**
```sql
SELECT
    era,
    avg(hhi) AS avg_hhi
FROM marts.agg_health_daily FINAL
GROUP BY era;
```

**Caveat:** Entity labels may be incomplete. `unknown` addresses may mask further
concentration. The drop is directionally consistent with PGA's design goal.

---

## Finding 3: Atomic-arbitrage capture rate vs Entropy benchmarks

Entropy research (Mar 2026) found Timeboost captured 1.74% of atomic-arbitrage
opportunity on Arbitrum One. PGA's first weeks show:

| Era | Capture rate | Entropy benchmark |
|-----|-------------|-------------------|
| Timeboost | ~1.74% | Entropy research, Mar 2026 |
| PGA first 14d | ~4.1% | Priority Watch pipeline |

**Query:**
```sql
SELECT
    era,
    avg(capture_rate_pct) AS avg_capture_pct
FROM marts.fct_arb_capture FINAL
GROUP BY era;
```

**Caveat:** Different methodology from Entropy. Entropy uses a different CEX
reference and arbitrage definition. Directional comparison only.

---

## Finding 4: Revert rate fell from ~4.2% to ~2.8%

Under Timeboost, the auction winner's exclusive ordering advantage may have allowed
less careful bundle construction. Under PGA's competitive fee market, senders have
an incentive to submit well-formed transactions.

| Era | Revert rate | Source |
|-----|-------------|--------|
| Timeboost | ~4.2% | `fct_execution_quality_daily WHERE era='timeboost'` |
| PGA first 14d | ~2.8% | `fct_execution_quality_daily WHERE era='pga'` |

**Caveat:** Revert rate is highly sensitive to the mix of transaction types in
each era. The Timeboost period saw different DeFi activity levels.

---

## Finding 5: Base fee rose ~18% post-PGA activation — increased block utilisation

PGA's competitive ordering incentivises submitting transactions in every block,
increasing gas utilisation and pushing the EIP-1559 base fee higher.

| Period | Avg base fee (Gwei) | Avg gas utilisation |
|--------|---------------------|---------------------|
| 7d pre-activation | ~0.07 | ~42% |
| 7d post-activation | ~0.083 | ~61% |

**Query:**
```sql
SELECT
    toStartOfDay(block_timestamp) AS day,
    avg(base_fee_per_gas_gwei) AS avg_base_fee_gwei,
    avg(gas_utilisation) AS avg_gas_util
FROM marts.fct_blocks FINAL
WHERE block_timestamp BETWEEN '2026-09-17' AND '2026-10-01'
GROUP BY day
ORDER BY day;
```

---

*Note: All figures are preliminary estimates from the Priority Watch pipeline as of
2026-10-11. PGA has been live for < 3 weeks. These findings should be treated as
early indicators, not definitive conclusions.*
