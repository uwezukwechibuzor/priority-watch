# Runbook: Re-running dbt After a Formula Change

When a metric formula changes (e.g., H1 tip hypothesis confirmed/refuted, new
entity labels, activation block update), all downstream models must be rebuilt.

## Steps

### 1. Update the dbt model
Edit the relevant model SQL in `dbt/models/`. Update the `description:` in the
corresponding `_models.yml` with the new formula and source.

### 2. Update CLAUDE.md section 11 (Canonical Fee Formulas)
The formula section must stay in sync with the dbt model.

### 3. Full rebuild from the changed model
```bash
# Rebuild the changed model and all descendants
cd dbt
dbt run --select <changed_model>+ --full-refresh
dbt test --select <changed_model>+
```

### 4. Verify the change
```bash
# Spot-check against a known block
dbt run --select fct_blocks --full-refresh
```

### 5. Commit and push
Ensure `CLAUDE.md`, the dbt model, and `_models.yml` are updated in the same commit.

## Common formula changes

| Change | Models affected |
|--------|----------------|
| H1 confirmed (tip on L2 gas only) | `int_block_fees`, all marts |
| Activation block updated | `stg_blocks` (era field), all marts |
| New entity label added | `int_entity_labels`, all marts |
| New DEX pool added | `stg_dex_swaps`, `fct_spread_per_block` |
