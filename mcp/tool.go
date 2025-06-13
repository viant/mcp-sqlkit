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
	if err := protoserver.RegisterTool[*query.Input, *query.Output](base.Registry, "dbQuery", "Execute SQL query and return result set as JSON array.", func(ctx context.Context, input *query.Input) (*schema.CallToolResult, *jsonrpc.Error) {
		out := ret.query.Query(ctx, input)
		if out.Status == "error" {
			return buildErrorResult(out.Error)
		}
		return buildSuccessResult(ret.service, out)
	}); err != nil {
		return err
	}



	// Register exec tool
	if err := protoserver.RegisterTool[*exec.Input, *exec.Output](base.Registry, "dbExec", "Execute SQL DML/DDL statement and return rows affected and last insert id.", func(ctx context.Context, input *exec.Input) (*schema.CallToolResult, *jsonrpc.Error) {
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

    // Register add connection tool (structured input, no DSN allowed)
    if err := protoserver.RegisterTool[*connector.ConnectionInput, *connector.AddOutput](base.Registry, "dbAddConnection", "Register a new database connector.", func(ctx context.Context, input *connector.ConnectionInput) (*schema.CallToolResult, *jsonrpc.Error) {
            if err := ret.connectors.UpsertConnection(ctx, input); err != nil {
	            return buildErrorResult(err.Error())
	        }
            out := &connector.AddOutput{Status: "ok", Connector: input.Name}
	        return buildSuccessResult(ret.service, out)
	}); err != nil {
	        return err
	}

    // Register update connection tool (upsert)
    if err := protoserver.RegisterTool[*connector.ConnectionInput, *connector.UpdateOutput](base.Registry, "dbUpdateConnection", "Update an existing connector (or create if absent).", func(ctx context.Context, input *connector.ConnectionInput) (*schema.CallToolResult, *jsonrpc.Error) {
            if err := ret.connectors.UpsertConnection(ctx, input); err != nil {
	            return buildErrorResult(err.Error())
	        }
            out := &connector.UpdateOutput{Status: "ok", Connector: input.Name}
	        return buildSuccessResult(ret.service, out)
	}); err != nil {
	        return err
	}

	// Register list tables tool
	if err := protoserver.RegisterTool[*meta.ListTablesInput, *meta.TablesOutput](base.Registry, "dbListTables", "List tables for the specified catalog/schema.", func(ctx context.Context, input *meta.ListTablesInput) (*schema.CallToolResult, *jsonrpc.Error) {
		out := ret.meta.ListTables(ctx, input)
		if out.Status == "error" {
			return buildErrorResult(out.Error)
		}
		return buildSuccessResult(ret.service, out)
	}); err != nil {
		return err
	}

	// Register list columns tool
	if err := protoserver.RegisterTool[*meta.ListColumnsInput, *meta.ColumnsOutput](base.Registry, "dbListColumns", "List columns for the specified table.", func(ctx context.Context, input *meta.ListColumnsInput) (*schema.CallToolResult, *jsonrpc.Error) {
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
