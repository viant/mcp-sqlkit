package mcp

import (
	"context"
	_ "embed"

	"github.com/viant/jsonrpc"
	"github.com/viant/mcp-protocol/schema"
	protoserver "github.com/viant/mcp-protocol/server"

	"github.com/viant/mcp-sqlkit/db/connector"
	"github.com/viant/mcp-sqlkit/db/exec"
	"github.com/viant/mcp-sqlkit/db/meta"
	"github.com/viant/mcp-sqlkit/db/query"
)

// Embedded markdown descriptions for tools
//
//go:embed descriptions/dbQuery.md
var dbQueryDesc string

//go:embed descriptions/dbExec.md
var dbExecDesc string

//go:embed descriptions/dbListConnections.md
var dbListConnectionsDesc string

//go:embed descriptions/dbSetConnection.md
var dbSetConnectionDesc string

//go:embed descriptions/dbListTables.md
var dbListTablesDesc string

//go:embed descriptions/dbListColumns.md
var dbListColumnsDesc string

func registerTools(base *protoserver.DefaultHandler, ret *Handler) error {
	// Register query tool
	if err := protoserver.RegisterTool[*query.Input, *query.Output](base.Registry, "dbQuery", dbQueryDesc, func(ctx context.Context, input *query.Input) (*schema.CallToolResult, *jsonrpc.Error) {
		out := ret.query.Query(ctx, input)
		if out.Status == "error" {
			return buildErrorResult(out.Error)
		}
		return buildSuccessResult(ret.service, out)
	}); err != nil {
		return err
	}

	// Register exec tool
	if err := protoserver.RegisterTool[*exec.Input, *exec.Output](base.Registry, "dbExec", dbExecDesc, func(ctx context.Context, input *exec.Input) (*schema.CallToolResult, *jsonrpc.Error) {
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
	if err := protoserver.RegisterTool[*connector.ListInput, *connector.ListOutput](base.Registry, "dbListConnections", dbListConnectionsDesc, func(ctx context.Context, input *connector.ListInput) (*schema.CallToolResult, *jsonrpc.Error) {
		out := ret.connectors.ListConnectors(ctx, input)
		if out.Status == "error" {
			return buildErrorResult(out.Error)
		}
		return buildSuccessResult(ret.service, out)
	}); err != nil {
		return err
	}

	// Register set connection tool (upsert + form + OOB secret elicitation)
	if err := protoserver.RegisterTool[*connector.ConnectionInput, *connector.AddOutput](base.Registry, "dbSetConnection", dbSetConnectionDesc, func(ctx context.Context, input *connector.ConnectionInput) (*schema.CallToolResult, *jsonrpc.Error) {
		out, err := ret.connectors.AddConnection(ctx, input)
		if err != nil {
			return buildErrorResult(err.Error())
		}
		return buildSuccessResult(ret.service, out)
	}); err != nil {
		return err
	}

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
	if err := protoserver.RegisterTool[*meta.ListColumnsInput, *meta.ColumnsOutput](base.Registry, "dbListColumns", dbListColumnsDesc, func(ctx context.Context, input *meta.ListColumnsInput) (*schema.CallToolResult, *jsonrpc.Error) {
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
