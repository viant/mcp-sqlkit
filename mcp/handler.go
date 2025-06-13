package mcp

import (
	"context"
	"github.com/viant/jsonrpc/transport"
	protoclient "github.com/viant/mcp-protocol/client"
	"github.com/viant/mcp-protocol/logger"
	protoserver "github.com/viant/mcp-protocol/server"
	"github.com/viant/mcp-sqlkit/db/connector"
	"github.com/viant/mcp-sqlkit/db/exec"
	"github.com/viant/mcp-sqlkit/db/meta"
	"github.com/viant/mcp-sqlkit/db/query"
)

type Handler struct {
	*protoserver.DefaultHandler
	service    *Service
	exec       *exec.Service
	query      *query.Service
	meta       *meta.Service
	connectors *connector.Service
}

func NewHandler(service *Service) protoserver.NewHandler {
	return func(_ context.Context, notifier transport.Notifier, logger logger.Logger, clientOperation protoclient.Operations) (protoserver.Handler, error) {
		base := protoserver.NewDefaultHandler(notifier, logger, clientOperation)
		ret := &Handler{
			DefaultHandler: base,
			service:        service,
			query:          service.NewQueryService(clientOperation),
			exec:           service.NewExecService(clientOperation),
			meta:           service.NewMetaService(clientOperation),
			connectors:     service.NewConnector(clientOperation),
		}
		err := registerTools(base, ret)
		if err != nil {
			return nil, err
		}
		return ret, nil
	}
}
