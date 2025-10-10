Register a new database connector when the requested database is not served by any known connector.

Elicitation Policy
- Never ask user to provide any connection details except connection name. Tool has ability to elicitate out of bound.


Shared Rules
- Never guess or reuse a connector for the wrong DB.
- Always validate target DB vs connectors from `dbListConnections`.

