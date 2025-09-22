package mcp

import (
	"context"

	"github.com/viant/jsonrpc"
	"github.com/viant/mcp-protocol/schema"
	protoserver "github.com/viant/mcp-protocol/server"

	"github.com/viant/mcp-sqlkit/db/connector"
	"github.com/viant/mcp-sqlkit/db/exec"
	"github.com/viant/mcp-sqlkit/db/meta"
	"github.com/viant/mcp-sqlkit/db/query"
)

func registerTools(base *protoserver.DefaultHandler, ret *Handler) error {
	// Register query tool
	if err := protoserver.RegisterTool[*query.Input, *query.Output](base.Registry, "dbQuery", "Execute SQL query and return result set as JSON array. If you don't know dsn use 'dev' Connector.", func(ctx context.Context, input *query.Input) (*schema.CallToolResult, *jsonrpc.Error) {
		out := ret.query.Query(ctx, input)
		if out.Status == "error" {
			return buildErrorResult(out.Error)
		}
		return buildSuccessResult(ret.service, out)
	}); err != nil {
		return err
	}

	// Register exec tool
	if err := protoserver.RegisterTool[*exec.Input, *exec.Output](base.Registry, "dbExec", "Execute SQL DML/DDL statement and return rows affected and last insert id. If you don't know dsn use 'dev' Connector", func(ctx context.Context, input *exec.Input) (*schema.CallToolResult, *jsonrpc.Error) {
		out := ret.exec.Execute(ctx, input)
		if out.Status == "error" {
			return buildErrorResult(out.Error)
		}
		// compact execution result (omit status field)
		summary := map[string]interface{}{"rowsAffected": out.RowsAffected, "lastInsertId": out.LastInsertId}
		return buildSuccessResult(ret.service, summary)
	}); err != nil {
		return err
	}

	// Register list connections tool
	if err := protoserver.RegisterTool[*connector.ListInput, *connector.ListOutput](base.Registry, "dbListConnections", "List database connectors.", func(ctx context.Context, input *connector.ListInput) (*schema.CallToolResult, *jsonrpc.Error) {
		out := ret.connectors.ListConnectors(ctx, input)
		if out.Status == "error" {
			return buildErrorResult(out.Error)
		}
		return buildSuccessResult(ret.service, out)
	}); err != nil {
		return err
	}

	// Register set connection tool (upsert + form + OOB secret elicitation)
	if err := protoserver.RegisterTool[*connector.ConnectionInput, *connector.AddOutput](base.Registry, "dbSetConnection", "Creates or updates a connector. If any connection detail is missing, a form is shown to collect it. Secrets are collected via a secure browser flow and never provided in-band. Partial inputs prefill the form.", func(ctx context.Context, input *connector.ConnectionInput) (*schema.CallToolResult, *jsonrpc.Error) {
		out, err := ret.connectors.AddConnection(ctx, input)
		if err != nil {
			return buildErrorResult(err.Error())
		}
		return buildSuccessResult(ret.service, out)
	}); err != nil {
		return err
	}

	dbListTablesDesc := `Lists tables/views from databases for the specified catalog/schema.
If you don't know the DSN, use the 'dev' Connector to initiate DSN elicitation.

Parameters:
- connector: string (required)
- catalog: string (required for BigQuery; forbidden for MySQL/Postgres)
- schema: string (required for MySQL/Postgres/BigQuery)

NEVER use unknown parameters (e.g., "table").

Returns:
{
  "status": "ok"|"error",
  "data": [{"Catalog":string,"Schema":string,"Name":string,"Type":"TABLE"|"VIEW","CreateTime":string (timestamp RFC3339)}]
}
`

	// Register list tables tool
	if err := protoserver.RegisterTool[*meta.ListTablesInput, *meta.TablesOutput](base.Registry, "dbListTables", dbListTablesDesc, func(ctx context.Context, input *meta.ListTablesInput) (*schema.CallToolResult, *jsonrpc.Error) {
		out := ret.meta.ListTables(ctx, input)
		if out.Status == "error" {
			return buildErrorResult(out.Error)
		}
		return buildSuccessResult(ret.service, out)
	}); err != nil {
		return err
	}

	// Register list columns tool
	if err := protoserver.RegisterTool[*meta.ListColumnsInput, *meta.ColumnsOutput](base.Registry, "dbListColumns", "List columns for the specified table. If you don't know dsn use 'dev' Connector to initiate dsn elicitation.", func(ctx context.Context, input *meta.ListColumnsInput) (*schema.CallToolResult, *jsonrpc.Error) {
		out := ret.meta.ListColumns(ctx, input)
		if out.Status == "error" {
			return buildErrorResult(out.Error)
		}
		return buildSuccessResult(ret.service, out)
	}); err != nil {
		return err
	}
	return nil
}
