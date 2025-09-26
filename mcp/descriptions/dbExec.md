Execute a SQL DML/DDL statement and return a compact execution summary.

Pre-Flight
- Confirm a connector that serves the target database via `dbListConnections`.
- Do not default or reuse a connector for a different database.

If Missing Connector
- Ask whether to add one using `dbSetConnection` and collect all required parameters at once.

Output
- `rowsAffected`: number of rows affected by the statement.
- `lastInsertId`: last inserted row ID when supported by the engine.

Shared Rules
- Never guess or reuse a connector for the wrong DB.
- Always validate against `dbListConnections` before calling.

