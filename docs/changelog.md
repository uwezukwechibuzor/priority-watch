# Changelog

## v1.0.0 — 2026-10-11

Initial public release of Priority Watch.

### Features
- **Ingester**: Block backfill, live block follower, Binance CEX klines, Timeboost baseline import
- **dbt pipeline**: Full staging -> intermediate -> marts transformation pipeline
- **API**: All endpoint groups (blocks, health, pricing, backtest, opportunity, treasury, compare, datasets)
- **Dashboard**: `/blocks`, `/health`, `/pricing`, `/opportunity`, `/simulator` pages
- **CI/CD**: GitHub Actions with Go, Python, web, docs, and release workflows
- **Docs**: MkDocs Material knowledge base with architecture, glossary, methodology, limitations, runbooks
- **Observability**: Prometheus + Grafana dashboards, Caddy reverse proxy with TLS

### Data coverage
- PGA era: ~2026-09-24 to present
- Timeboost baseline: 2025-04-17 to ~2026-09-24
