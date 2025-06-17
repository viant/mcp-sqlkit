package meta

import (
	"context"
	"database/sql"
	"net/url"
	"strings"

	// Ensure popular drivers & metadata products are registered.
	_ "github.com/viant/mcp-sqlkit/db/driver"

	"github.com/viant/mcp-sqlkit/db/connector"
	"github.com/viant/sqlx/metadata"
	"github.com/viant/sqlx/metadata/info"
	"github.com/viant/sqlx/metadata/sink"
	"github.com/viant/sqlx/option"
)

// ListTablesInput defines parameters for retrieving table metadata.
type ListTablesInput struct {
	// Connector name registered in the toolbox.
	Connector string `json:"connector,omitempty"`

	// Catalog/database name (optional).
	Catalog string `json:"catalog,omitempty"`

	// Schema name (optional – defaults depend on the driver).
	Schema string `json:"schema,omitempty"`
}

// ListColumnsInput defines parameters for retrieving column metadata of a table.
type ListColumnsInput struct {
	// Connector to use.
	Connector string `json:"connector,omitempty"`

	// Catalog/database name (optional).
	Catalog string `json:"catalog,omitempty"`

	// Schema name (optional).
	Schema string `json:"schema,omitempty"`

	// Table for which columns should be listed.
	Table string `json:"table"`
}

// Output follows the same envelope format used by other DB tools.

// TablesOutput wraps table metadata.
type TablesOutput struct {
	Data      []sink.Table `json:"data,omitempty"`
	Status    string       `json:"status"`
	Error     string       `json:"error,omitempty"`
	Connector string       `json:"connector,omitempty"`
}

// ColumnsOutput wraps column metadata.
type ColumnsOutput struct {
	Data      []sink.Column `json:"data,omitempty"`
	Status    string        `json:"status"`
	Error     string        `json:"error,omitempty"`
	Connector string        `json:"connector,omitempty"`
}

// Service provides metadata listing capabilities.
type Service struct {
	connectors *connector.Service
}

// New creates a metadata service instance.
func New(connectors *connector.Service) *Service {
	return &Service{connectors: connectors}
}

// ListTables returns tables available in the specified catalog/schema.
func (s *Service) ListTables(ctx context.Context, input *ListTablesInput) *TablesOutput {
	out := &TablesOutput{Status: "ok"}
	if err := s.listTables(ctx, input, out); err != nil {
		out.Status = "error"
		out.Error = err.Error()
	}
	return out
}

// ListColumns returns column metadata for the specified table.
func (s *Service) ListColumns(ctx context.Context, input *ListColumnsInput) *ColumnsOutput {
	out := &ColumnsOutput{Status: "ok"}

	if err := s.listColumns(ctx, input, out); err != nil {
		out.Status = "error"
		out.Error = err.Error()
	}
	return out
}

// listTables executes the metadata query and fills the output on success.
func (s *Service) listTables(ctx context.Context, input *ListTablesInput, out *TablesOutput) error {
	db, err := s.db(ctx, input.Connector)
	if err != nil {
		return err
	}

	if input.Schema == "" {
		input.Schema = s.extractSchema(ctx, input.Connector)
	}
	m := metadata.New()
	var tables []sink.Table
	if err := m.Info(ctx, db, info.KindTables, &tables, option.NewArgs(input.Catalog, input.Schema)); err != nil {
		return err
	}

	out.Connector = input.Connector
	out.Data = tables
	return nil
}

func (s *Service) extractSchema(ctx context.Context, connector string) string {
	conn, _ := s.connectors.Connection(ctx, connector)
	if URL, err := url.Parse(conn.DSN); err == nil {
		return strings.Trim(URL.Path, "/")
	}
	return ""
}

// listColumns executes the metadata query and fills the output on success.
func (s *Service) listColumns(ctx context.Context, input *ListColumnsInput, out *ColumnsOutput) error {
	db, err := s.db(ctx, input.Connector)
	if err != nil {
		return err
	}
	if input.Schema == "" {
		input.Schema = s.extractSchema(ctx, input.Connector)
	}
	m := metadata.New()
	var columns []sink.Column
	if err := m.Info(ctx, db, info.KindTable, &columns, option.NewArgs(input.Catalog, input.Schema, input.Table)); err != nil {
		return err
	}

	out.Connector = input.Connector
	out.Data = columns
	return nil
}

// db resolves the *sql.DB instance for a connector.
func (s *Service) db(ctx context.Context, name string) (*sql.DB, error) {
	conn, err := s.connectors.Connection(ctx, name)
	if err != nil {
		return nil, err
	}
	return conn.Db(ctx)
}
