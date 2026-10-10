-- Override dbt's default schema-name generation so that +schema: staging in
-- dbt_project.yml resolves to the ClickHouse database named exactly "staging",
-- not "{target.schema}__staging".  Without this override dbt prepends the
-- profile's schema value, which would give "staging__staging".
{% macro generate_schema_name(custom_schema_name, node) -%}
    {%- if custom_schema_name is not none -%}
        {{ custom_schema_name | trim }}
    {%- else -%}
        {{ target.schema | trim }}
    {%- endif -%}
{%- endmacro %}
