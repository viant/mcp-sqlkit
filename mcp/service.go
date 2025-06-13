package mcp

import (
	"net/http"

	"github.com/viant/mcp-protocol/client"
	"github.com/viant/mcp-sqlkit/auth"
	"github.com/viant/mcp-sqlkit/db/connector"
	"github.com/viant/mcp-sqlkit/db/exec"
	"github.com/viant/mcp-sqlkit/db/meta"
	"github.com/viant/mcp-sqlkit/db/query"
	"github.com/viant/mcp-sqlkit/mcp/ui/interaction"
	"github.com/viant/mcp-sqlkit/policy"
	"github.com/viant/scy"
)

type Service struct {
	connectors *connector.Manager
	ui         *interaction.Service
	auth       *auth.Service

	// useText determines which field (`text` vs `data`) the toolbox will
	// populate when returning CallToolResultContentElem.
	useText bool
}

// RegisterHTTP attaches all MCP auxiliary HTTP handlers (currently user-interaction callbacks).
func (s *Service) RegisterHTTP(mux *http.ServeMux) {
	if s.ui != nil {
		s.ui.Register(mux)
	}
}

func (s *Service) NewQueryService(operations client.Operations) *query.Service {
	return query.New(s.NewConnector(operations))
}

func (s *Service) NewExecService(operations client.Operations) *exec.Service {
	return exec.New(s.NewConnector(operations))
}

func (s *Service) NewMetaService(operations client.Operations) *meta.Service {
	return meta.New(s.NewConnector(operations))
}

func (s *Service) NewConnector(operations client.Operations) *connector.Service {
	return connector.NewService(s.connectors, operations)
}

func (s *Service) UI() *interaction.Service {
	return s.ui
}

// Auth returns the underlying authentication service.
func (s *Service) Auth() *auth.Service {
	return s.auth
}

// UseTextField indicates whether SQLKit should populate the `text` field
// (true – default) or the `data` field (false) when returning tool results.
func (s *Service) UseTextField() bool {
	return s.useText
}

func NewService(config *Config) *Service {
	if config == nil {
		config = &Config{}
	}

	// Ensure nested structs are initialised to avoid nil-pointer dereferences.
	if config.Connector == nil {
		config.Connector = &connector.Config{}
	}

	if config.Connector.Policy == nil {
		config.Connector.Policy = &policy.Policy{}
	}

	authService := auth.New(config.Connector.Policy)
	secrets := scy.New()
	connectors := connector.NewManager(config.Connector, authService, secrets)

    // Determine field preference for tool results.
    useText := true // default – place JSON in `text`
    if config.UseData {
        useText = false
    } else if config.UseText { // legacy opt-in flag
        useText = true
    }

    ret := &Service{
        connectors: connectors,
        ui:         interaction.New(connectors, secrets),
        auth:       authService,
        useText:    useText,
    }
	return ret
}
