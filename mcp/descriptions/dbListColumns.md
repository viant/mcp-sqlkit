List columns for the specified table.

Pre-Flight
- Parse the full identifier (db.schema.table if provided).
- Confirm a connector that serves the database via `dbListConnections`.
- Never call with a connector that does not serve the table's database.

If Missing Connector
- Offer to add one via `dbSetConnection` with a full one-shot form.

Shared Rules
- Never guess or reuse a connector for the wrong DB.
- Always validate against `dbListConnections` before calling.

