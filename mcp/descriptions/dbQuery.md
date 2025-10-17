Execute a read-only SQL query and return the result set as JSON.
Avoid large     result use LIMIT or OFFSET based on driver

Pre-Flight
- Confirm a connector that serves the database referenced by the query.
- Use `dbListConnections` if uncertain; do not guess or default a connector.

If Missing Connector
- Ask whether to add one using `dbSetConnection`.
- Collect all required fields at once (a one-shot form), not piecemeal.

Output
- On success: JSON array of rows.
- On error: descriptive error message.

Shared Rules
- Never guess or reuse a connector for the wrong DB.
- Always validate against `dbListConnections` before calling.

