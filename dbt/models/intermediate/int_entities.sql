{{
    config(
        materialized = 'table',
        order_by = 'address',
        engine = 'ReplacingMergeTree'
    )
}}

-- Entity address mapping for PGA analytics.
-- Sources (priority order, first match wins after deduplication):
--   1. entities seed — verified participant labels (searchers, MMs, protocols)
--   2. pools seed    — DEX pool contracts (entity_name = "<dex> <token0>/<token1>")
--   3. contracts seed — reference contracts (Aave, Uniswap factory, etc.)
--   4. heuristic bots — EOAs that call the same (contract, selector) 3+ times in a
--      single block; label_source = 'heuristic', confidence = 0.5.
--
-- The heuristic detects burst-bot behaviour common in PGA rounds but will produce
-- false positives for any high-frequency wallet. All heuristic rows should be
-- treated as candidates, not confirmed entities.
WITH seed_entities AS (
    SELECT
        lower(address)                          AS address,
        entity_name,
        entity_type,
        toUInt8(color_index)                    AS color_index,
        address_label,
        toUInt8(verified)                       AS verified,
        'seed'                                  AS label_source,
        toFloat32(1.0)                          AS confidence
    FROM {{ ref('entities') }}
    WHERE address != ''
),

pool_entities AS (
    SELECT
        lower(address)                                              AS address,
        concat(dex, ' ', token0_symbol, '/', token1_symbol)        AS entity_name,
        'protocol'                                                  AS entity_type,
        toUInt8(0)                                                  AS color_index,
        concat(dex, ' pool')                                        AS address_label,
        toUInt8(verified)                                           AS verified,
        'seed'                                                      AS label_source,
        toFloat32(1.0)                                              AS confidence
    FROM {{ ref('pools') }}
    WHERE address != ''
),

contract_entities AS (
    SELECT
        lower(address)                          AS address,
        name                                    AS entity_name,
        'protocol'                              AS entity_type,
        toUInt8(0)                              AS color_index,
        name                                    AS address_label,
        toUInt8(verified)                       AS verified,
        'seed'                                  AS label_source,
        toFloat32(1.0)                          AS confidence
    FROM {{ ref('contracts') }}
    WHERE address != ''
),

-- Heuristic: EOAs sending 3+ txs to the same contract+selector in a single block.
-- Uses int_tx_fees which already has from_address and to_address.
heuristic_bots AS (
    SELECT
        f.from_address                          AS address,
        concat('Bot (', f.to_address, ')')      AS entity_name,
        'searcher'                              AS entity_type,
        toUInt8(0)                              AS color_index,
        'heuristic bot'                         AS address_label,
        toUInt8(0)                              AS verified,
        'heuristic'                             AS label_source,
        toFloat32(0.5)                          AS confidence
    FROM {{ ref('int_tx_fees') }} FINAL AS f
    WHERE f.to_address != ''
        AND f.input_selector != ''
    GROUP BY
        f.block_number,
        f.from_address,
        f.to_address,
        f.input_selector
    HAVING count() >= 3
),

-- Union all sources; dedup by address keeping highest-confidence / first-priority source
all_labels AS (
    SELECT *, 1 AS priority FROM seed_entities
    UNION ALL
    SELECT *, 2 AS priority FROM pool_entities
    UNION ALL
    SELECT *, 3 AS priority FROM contract_entities
    UNION ALL
    SELECT *, 4 AS priority FROM heuristic_bots
)

SELECT
    address,
    argMin(entity_name, priority)       AS entity_name,
    argMin(entity_type, priority)       AS entity_type,
    argMin(color_index, priority)       AS color_index,
    argMin(address_label, priority)     AS address_label,
    argMin(verified, priority)          AS verified,
    argMin(label_source, priority)      AS label_source,
    argMin(confidence, priority)        AS confidence
FROM all_labels
GROUP BY address
