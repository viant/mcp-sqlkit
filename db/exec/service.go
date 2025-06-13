package exec

import (
	"context"

	"github.com/viant/mcp-protocol/client"
	"github.com/viant/mcp-sqlkit/db/connector"
)

type Input struct {
	Query      string
	Connector  string
	Parameters []interface{}
}

type Output struct {
	RowsAffected int64  `json:"rowsAffected,omitempty"`
	LastInsertId int64  `json:"LastInsertId,omitempty"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
	Connector    string `json:",omitempty"`
}

type Service struct {
	connectors *connector.Service
	operation  client.Operations
}

func (r *Service) Execute(ctx context.Context, input *Input) *Output {
	output := &Output{Status: "ok"}
	err := r.execute(ctx, input, output)
	if err != nil {
		output.Error = err.Error()
		output.Status = "error"
	}
	return output
}

func (r *Service) execute(ctx context.Context, input *Input, output *Output) error {
	con, err := r.connectors.Connection(ctx, input.Connector)
	if err != nil {
		return err
	}
	db, err := con.Db(ctx)
	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, input.Query, input.Parameters)
	if err != nil {
		return err
	}
	output.RowsAffected, _ = result.RowsAffected()
	output.LastInsertId, _ = result.LastInsertId()
	return nil
}

func New(connectors *connector.Service) *Service {
	return &Service{connectors: connectors}
}
