List tables/views for a specific database/schema.

Pre-Flight
- Identify the target database and schema.
- Confirm a connector that serves it via `dbListConnections`.
- Do not call with a connector tied to a different database.

If Missing Connector
- Ask whether to add one via `dbSetConnection`.
- Collect all required fields at once (one-shot form), never a single field.

Shared Rules
- Never guess or reuse a connector for the wrong DB.
- Always validate against `dbListConnections` before calling.

