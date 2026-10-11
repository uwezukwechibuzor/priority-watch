# Runbook: Secret Rotation

Secrets are stored in `.env` (gitignored). They are injected as environment
variables into Docker Compose services.

## Rotate the Arbitrum RPC key

1. Obtain a new API key from your RPC provider.
2. Update `.env`:
   ```
   ARBITRUM_RPC_URL=https://arb-mainnet.g.alchemy.com/v2/NEW_KEY
   ARBITRUM_WS_URL=wss://arb-mainnet.g.alchemy.com/v2/NEW_KEY
   ```
3. Restart affected services:
   ```bash
   docker compose restart pw-follower
   ```
4. Verify: `make smoke`

## Rotate the ClickHouse password

1. Update `.env`:
   ```
   CLICKHOUSE_PASSWORD=new-strong-password
   ```
2. Update the password inside ClickHouse:
   ```sql
   ALTER USER default IDENTIFIED WITH sha256_password BY 'new-strong-password';
   ```
3. Restart all services that connect to ClickHouse:
   ```bash
   docker compose restart
   ```

## Rotate the Airflow Fernet key

1. Generate a new Fernet key:
   ```bash
   python3 -c "from cryptography.fernet import Fernet; print(Fernet.generate_key().decode())"
   ```
2. Update `.env`: `AIRFLOW_FERNET_KEY=<new_key>`
3. Re-encrypt existing connections:
   ```bash
   docker compose exec airflow-webserver airflow connections rotate-fernet-key
   ```
4. Restart: `docker compose restart airflow-webserver airflow-scheduler`

## Rotate the Airflow webserver secret key

1. Generate: `python3 -c "import secrets; print(secrets.token_hex(32))"`
2. Update `.env`: `AIRFLOW_SECRET_KEY=<new_key>`
3. Restart: `docker compose restart airflow-webserver`

## Secrets checklist

| Secret | Location | Rotation frequency |
|--------|----------|--------------------|
| `ARBITRUM_RPC_URL` | `.env` | On provider key expiry |
| `CLICKHOUSE_PASSWORD` | `.env` | Quarterly |
| `AIRFLOW_FERNET_KEY` | `.env` | Annually |
| `AIRFLOW_SECRET_KEY` | `.env` | Annually |
| `SLACK_WEBHOOK_URL` | `.env` | On Slack workspace change |
