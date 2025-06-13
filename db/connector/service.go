package connector

import (
    "context"
    "errors"
    "fmt"

    "github.com/viant/mcp-protocol/client"
    "github.com/viant/mcp-protocol/schema"
    "github.com/viant/mcp-sqlkit/db/connector/meta"
)

type Service struct {
	*Manager
	mcpClient client.Operations
}

// UpsertConnection registers or updates a connector based on structured input
// that deliberately excludes credential fields. The method validates the
// input against driver metadata, expands the server-side DSN template and
// delegates to Service.Add which handles secret elicitation.
func (s *Service) UpsertConnection(ctx context.Context, input *ConnectionInput) error {
    if input == nil {
        return fmt.Errorf("connection input cannot be nil")
    }

    // Obtain driver metadata to fill defaults and validate.
    metaCfg := s.matchMeta(input.Driver)
    input.Init(metaCfg)
    if err := input.Validate(metaCfg); err != nil {
        return err
    }

    // Expand DSN (place-holders like ${Host}, ${Port}, …).
    dsn := input.Expand(metaCfg.DSN)

    conn := &Connector{
        Name:   input.Name,
        Driver: input.Driver,
        DSN:    dsn,
    }

    return s.Add(ctx, conn)
}

func (s *Service) Connection(ctx context.Context, name string) (*Connector, error) {
	conn, err := s.Manager.Connection(ctx, name)
	if err == nil {
		return conn, nil
	}
	// Intercept typed errors to optionally trigger elicitation
	if errors.Is(err, ErrNamespaceNotFound) || errors.Is(err, ErrConnectorNotFound) && s.mcpClient != nil && s.mcpClient.Implements(schema.MethodElicitationCreate) {
		namespace, _ := s.auth.Namespace(ctx)
		name, err = s.requestConnectorElicit(ctx, s.mcpClient, name, namespace)
		if err != nil {
			return nil, err
		}
		// attempt again – return whatever result we get (including error)
		return s.Manager.Connection(ctx, name)
	}
	return nil, err
}

// matchMeta is kept for backward-compatibility – it simply delegates to the
// Manager implementation.
func (s *Service) matchMeta(driver string) *meta.Config {
	return s.Manager.matchMeta(driver)
}

// NewService builds a connector Service
func NewService(manager *Manager, mcp client.Operations) *Service {
	return &Service{
		Manager:   manager,
		mcpClient: mcp,
	}
}
