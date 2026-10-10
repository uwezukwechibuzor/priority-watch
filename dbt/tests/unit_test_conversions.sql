-- Singular test: validates all four conversion macros against known values.
-- Returns rows only when a macro produces a wrong result; the test passes
-- when this query returns zero rows.
--
-- Known values used:
--   hex_to_uint256('0x64')  → 100
--   hex_to_uint256('0x00')  → 0
--   hex_to_int256('0xffff...ff') → -1  (two's complement all-ones)
--   wei_to_eth(1000000000000000000) → 1.0
--   to_gwei(1000000000)             → 1.0
WITH checks AS (
    SELECT
        'hex_to_uint256_0x64'   AS test_name,
        toFloat64({{ hex_to_uint256("'0x64'") }})  AS got,
        100.0                                       AS want

    UNION ALL SELECT
        'hex_to_uint256_zero',
        toFloat64({{ hex_to_uint256("'0x00'") }}),
        0.0

    UNION ALL SELECT
        'hex_to_int256_neg_one',
        toFloat64({{ hex_to_int256("'0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff'") }}),
        -1.0

    UNION ALL SELECT
        'wei_to_eth_1e18',
        {{ wei_to_eth(1000000000000000000) }},
        1.0

    UNION ALL SELECT
        'to_gwei_1e9',
        {{ to_gwei(1000000000) }},
        1.0
)

SELECT
    test_name,
    got,
    want,
    abs(got - want) AS delta
FROM checks
WHERE abs(got - want) > 1e-9
