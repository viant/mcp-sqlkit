Register a new database connector when the requested database is not served by any known connector.

Elicitation Policy
- Ask the user to provide only connector name, all other parameters will be elicitated by a tool out of bound.

Shared Rules
- Never guess or reuse a connector for the wrong DB.
- Always validate target DB vs connectors from `dbListConnections`.

