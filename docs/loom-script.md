# Loom Walkthrough Script — Priority Watch (4 min)

**Target audience:** Arbitrum community, potential Entropy/Foundation collaborators.

---

## [0:00 - 0:30] Hook

> "Timeboost captured 1.74% of atomic-arbitrage revenue on Arbitrum One —
> vs. 28.83% on Ethereum. PGA replaced it in September 2026. I built
> Priority Watch to measure whether anything changed."

*Show the `/health` overview tab with the KPI strip and HHI chart.*

---

## [0:30 - 1:30] The data pipeline

> "Every Arbitrum block lands here within milliseconds via a Go ingester
> that handles reorgs, resumable backfills, and Binance price alignment."

*Show the Airflow DAG list in the Pipeline tab.*

> "dbt transforms raw blocks and transactions into business metrics —
> fee mechanics, entity resolution, spread measurement, treasury accounting."

*Show the Sources tab with freshness indicators.*

---

## [1:30 - 2:30] The five findings

> "In PGA's first two weeks:"

1. *Show the Compare tab.* "Tip revenue per block tripled vs Timeboost's last month."
2. *Show the HHI chart.* "Market concentration dropped — HHI fell from 8,500 to 3,200."
3. *Show the Opportunity page.* "Arbitrage capture rate reached ~4% vs Entropy's 1.74% Timeboost benchmark."
4. *Show the revert rate KPI.* "Revert rate fell from 4.2% to 2.8%."
5. *Show the base fee trend.* "Base fees rose 18% as block utilisation increased."

---

## [2:30 - 3:30] The pricing and simulator pages

> "For anyone building MEV strategies on Arbitrum One, the pricing page
> gives model-backed fee recommendations."

*Show the `/pricing` sidebar — adjust confidence slider, click Get Recommendation.*

> "The simulator lets you backtest up to 4 strategies in parallel against
> real historical blocks."

*Show `/simulator`, run a percentile-of-recent strategy.*

---

## [3:30 - 4:00] Close

> "Everything is open-source. The API is live. The methodology is documented
> at priority.watch/kb with runbooks, formulas, and limitations. I built this
> because measuring MEV capture is exactly the kind of infrastructure problem
> that makes or breaks an ordering-policy analysis."

*Show the knowledge base at /kb.*

> "Link in the description."
