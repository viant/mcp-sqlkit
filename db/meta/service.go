package meta

import (
	"context"
	"database/sql"
	"log"
	"net/url"
	"strings"
	"time"

	// Ensure popular drivers & metadata products are registered.
	_ "github.com/viant/mcp-sqlkit/db/driver"

	"github.com/viant/mcp-sqlkit/db/connector"
	"github.com/viant/sqlx/metadata"
	"github.com/viant/sqlx/metadata/database"
	"github.com/viant/sqlx/metadata/info"
	bigqueryproduct "github.com/viant/sqlx/metadata/product/bigquery"
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
	if input == nil {
		input = &ListTablesInput{}
	}
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
	if input == nil {
		input = &ListColumnsInput{}
	}
	if err := s.listColumns(ctx, input, out); err != nil {
		out.Status = "error"
		out.Error = err.Error()
	}
	return out
}

// listTables executes the metadata query and fills the output on success.
func (s *Service) listTables(ctx context.Context, input *ListTablesInput, out *TablesOutput) error {
	requestedConnector := input.Connector
	conn, db, err := s.connection(ctx, requestedConnector)
	if err != nil {
		return err
	}

	if input.Schema == "" {
		input.Schema = extractSchema(conn)
	}
	m := metadata.New()
	var tables []sink.Table
	started := time.Now()
	log.Printf("mcp-sqlkit dbListTables start connector=%q driver=%q catalog=%q schema=%q", requestedConnector, conn.Driver, input.Catalog, input.Schema)
	if err := m.Info(ctx, db, info.KindTables, &tables, s.metadataOptions(conn, input.Catalog, input.Schema)...); err != nil {
		log.Printf("mcp-sqlkit dbListTables error connector=%q driver=%q catalog=%q schema=%q elapsed=%s err=%v", requestedConnector, conn.Driver, input.Catalog, input.Schema, time.Since(started), err)
		return err
	}
	log.Printf("mcp-sqlkit dbListTables done connector=%q driver=%q catalog=%q schema=%q tables=%d elapsed=%s", requestedConnector, conn.Driver, input.Catalog, input.Schema, len(tables), time.Since(started))

	out.Connector = requestedConnector
	out.Data = tables
	return nil
}

func extractSchema(conn *connector.Connector) string {
	if conn == nil {
		return ""
	}
	if URL, err := url.Parse(conn.DSN); err == nil {
		return strings.Trim(URL.Path, "/")
	}
	return ""
}

// listColumns executes the metadata query and fills the output on success.
func (s *Service) listColumns(ctx context.Context, input *ListColumnsInput, out *ColumnsOutput) error {
	requestedConnector := input.Connector
	conn, db, err := s.connection(ctx, requestedConnector)
	if err != nil {
		return err
	}
	if input.Schema == "" {
		input.Schema = extractSchema(conn)
	}
	m := metadata.New()
	var columns []sink.Column
	started := time.Now()
	log.Printf("mcp-sqlkit dbListColumns start connector=%q driver=%q catalog=%q schema=%q table=%q", requestedConnector, conn.Driver, input.Catalog, input.Schema, input.Table)
	if err := m.Info(ctx, db, info.KindTable, &columns, s.metadataOptions(conn, input.Catalog, input.Schema, input.Table)...); err != nil {
		log.Printf("mcp-sqlkit dbListColumns error connector=%q driver=%q catalog=%q schema=%q table=%q elapsed=%s err=%v", requestedConnector, conn.Driver, input.Catalog, input.Schema, input.Table, time.Since(started), err)
		return err
	}
	log.Printf("mcp-sqlkit dbListColumns done connector=%q driver=%q catalog=%q schema=%q table=%q columns=%d elapsed=%s", requestedConnector, conn.Driver, input.Catalog, input.Schema, input.Table, len(columns), time.Since(started))

	out.Connector = requestedConnector
	out.Data = columns
	return nil
}

func (s *Service) metadataOptions(conn *connector.Connector, args ...interface{}) []option.Option {
	options := []option.Option{option.NewArgs(args...)}
	if product := metadataProduct(conn); product != nil {
		options = append(options, product)
	}
	return options
}

func metadataProduct(conn *connector.Connector) *database.Product {
	if conn == nil {
		return nil
	}
	switch strings.ToLower(conn.Driver) {
	case "bigquery":
		return bigqueryproduct.BigQuery()
	}
	return nil
}

func (s *Service) connection(ctx context.Context, name string) (*connector.Connector, *sql.DB, error) {
	conn, err := s.connectors.Connection(ctx, name)
	if err != nil {
		return nil, nil, err
	}
	db, err := conn.Db(ctx)
	if err != nil {
		return nil, nil, err
	}
	return conn, db, nil
}

// db resolves the *sql.DB instance for a connector.
func (s *Service) db(ctx context.Context, name string) (*sql.DB, error) {
	conn, err := s.connectors.Connection(ctx, name)
	if err != nil {
		return nil, err
	}
	return conn.Db(ctx)
}
