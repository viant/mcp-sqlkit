List available SQL connectors and the databases/schemas each can reach.

Usage
- Use when a user mentions a database but no connector is yet confirmed.
- Do not assume or default a connector.
- If multiple connectors can serve the DB, ask the user to choose.

Elicitation
- If no connector serves the requested DB, ask whether to add one via `dbSetConnection`.
- Collect all required parameters in a single message (a one-shot form), never piecemeal.

Shared Rules
- Never guess or reuse a connector for the wrong DB.
- Always validate against the list returned by this tool.

