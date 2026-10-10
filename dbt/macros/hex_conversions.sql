-- hex_to_uint256: interpret a big-endian hex string (with or without 0x prefix)
-- as an unsigned 256-bit integer.
--
-- Strategy: strip 0x, lpad to 64 hex chars (32 bytes), unhex to raw bytes,
-- reverse to little-endian, then reinterpretAsUInt256.
--
-- Unit-tested in tests/unit_test_conversions.sql.
{% macro hex_to_uint256(column) -%}
reinterpretAsUInt256(reverse(unhex(
    lpad(
        if(startsWith(toString({{ column }}), '0x'),
           substring(toString({{ column }}), 3),
           toString({{ column }})),
        64, '0')
)))
{%- endmacro %}


-- hex_to_int256: interpret a big-endian hex string as a signed 256-bit integer
-- using two's complement (identical byte layout to hex_to_uint256, different cast).
--
-- Unit-tested in tests/unit_test_conversions.sql.
{% macro hex_to_int256(column) -%}
reinterpretAsInt256(reverse(unhex(
    lpad(
        if(startsWith(toString({{ column }}), '0x'),
           substring(toString({{ column }}), 3),
           toString({{ column }})),
        64, '0')
)))
{%- endmacro %}
