-- wei_to_eth: convert a wei amount (numeric) to ETH as Float64.
-- Precision is sufficient for display; never use Float64 for accounting totals.
--
-- Unit-tested in tests/unit_test_conversions.sql.
{% macro wei_to_eth(column) -%}
toFloat64({{ column }}) / 1e18
{%- endmacro %}


-- to_gwei: convert a wei amount (numeric) to Gwei as Float64.
--
-- Unit-tested in tests/unit_test_conversions.sql.
{% macro to_gwei(column) -%}
toFloat64({{ column }}) / 1e9
{%- endmacro %}
