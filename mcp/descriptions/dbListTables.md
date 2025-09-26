Lists tables/views from databases for the specified catalog/schema.
If you don't know the DSN, use the 'dev' Connector to initiate DSN elicitation. Collect required information from listing database connectors

Parameters:
- connector: string (required)
- catalog: string (MUST populate  for BigQuery; CAN NOT use for MySQL/Postgres)
- schema: string (required for MySQL/Postgres/BigQuery)

NEVER use unknown parameters (e.g., "table").

Returns:
{
  "status": "ok"|"error",
  "data": [{"Catalog":string,"Schema":string,"Name":string,"Type":"TABLE"|"VIEW","CreateTime":string (timestamp RFC3339)}]
}
