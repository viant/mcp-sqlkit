package connector

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/viant/jsonrpc"
	"github.com/viant/mcp-protocol/client"
	"github.com/viant/mcp-protocol/schema"
	"github.com/viant/mcp-sqlkit/auth"
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
	// Backward-compatible wrapper that drives the new AddConnection flow and
	// discards the richer output.
	_, err := s.AddConnection(ctx, input)
	return err
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

// AddConnection orchestrates creation/upsert of a connector with a two-step
// elicitation flow: (1) form for non-secret parameters when missing; (2) out-of-
// band browser flow for secrets. It returns an AddOutput describing the state.
func (s *Service) AddConnection(ctx context.Context, input *ConnectionInput) (*AddOutput, error) {
	if input == nil {
		return nil, fmt.Errorf("connection input cannot be nil")
	}

	// If any required meta field is missing (including driver), elicit via form
	// when client supports MCP Elicit.
	needsForm := s.needsForm(ctx, input)
	if needsForm {
		impl := s.mcpClient
		if impl == nil || !impl.Implements(schema.MethodElicitationCreate) {
			return nil, fmt.Errorf("client does not support MCP Elicit; provide all required fields: name, driver and driver-specific parameters")
		}
		// Run form elicitation to collect/confirm non-secret parameters.
		name, err := s.requestConnectorForm(ctx, impl, input)
		if err != nil {
			return nil, err
		}
		// If form-based add succeeded without needing secrets, return ok.
		// The requestConnectorForm delegates to Add which handles the secret
		// flow – but it does not return output. To make sure we return state,
		// re-read the connector by name to build a simple ok output.
		return &AddOutput{Status: "ok", Connector: name}, nil
	}

	// Driver present and all required fields provided – validate/expand and
	// proceed directly to OOB secret elicitation.
	metaCfg := s.matchMeta(input.Driver)
	input.Init(metaCfg)
	if err := input.Validate(metaCfg); err != nil {
		return nil, err
	}
	dsn := input.Expand(metaCfg.DSN)
	conn := &Connector{Name: input.Name, Driver: input.Driver, DSN: dsn}
	return s.Set(ctx, conn)
}

// needsForm determines if any required non-secret parameters are missing.
func (s *Service) needsForm(ctx context.Context, in *ConnectionInput) bool {
	if in.Name == "" || in.Driver == "" {
		return true
	}
	metaCfg := s.matchMeta(in.Driver)
	dsn := metaCfg.DSN
	// Check placeholders and corresponding fields.
	if strings.Contains(dsn, "${Host}") && in.Host == "" {
		return true
	}
	if strings.Contains(dsn, "${Port}") && in.Port == 0 {
		return true
	}
	if strings.Contains(dsn, "${Db}") && in.Db == "" {
		return true
	}
	if strings.Contains(dsn, "${Project}") && in.Project == "" {
		return true
	}
	return false
}

// requestConnectorForm elicits a form to collect/confirm non-secret params and
// upon acceptance expands DSN and triggers secret OOB flow via Add(). It returns
// the connector name on success.
func (s *Service) requestConnectorForm(ctx context.Context, impl client.Operations, initial *ConnectionInput) (string, error) {
	// Build schema with dynamic required fields based on DSN placeholders – when
	// driver is missing, default required are just name/driver.
	props, _ := schema.StructToProperties(reflect.TypeOf(ConnectionInput{}))
	flatProps := make(map[string]interface{}, len(props))
	for k, v := range props {
		flatProps[k] = v
	}

	required := []string{"name", "driver"}
	// Pre-fill defaults using initial input.
	if initial != nil {
		if initial.Name != "" {
			if p, ok := flatProps["name"].(map[string]interface{}); ok {
				p["default"] = initial.Name
			}
		}
		if initial.Driver != "" {
			if p, ok := flatProps["driver"].(map[string]interface{}); ok {
				p["default"] = initial.Driver
			}
			// incorporate driver-specific required placeholders
			metaCfg := s.matchMeta(initial.Driver)
			dsn := metaCfg.DSN
			if strings.Contains(dsn, "${Host}") {
				required = append(required, "host")
			}
			if strings.Contains(dsn, "${Port}") {
				required = append(required, "port")
			}
			if strings.Contains(dsn, "${Db}") {
				required = append(required, "db")
			}
			if strings.Contains(dsn, "${Project}") {
				required = append(required, "project")
			}
		}
		// Apply provided values as defaults for better UX.
		if initial.Host != "" {
			if p, ok := flatProps["host"].(map[string]interface{}); ok {
				p["default"] = initial.Host
			}
		}
		if initial.Port != 0 {
			if p, ok := flatProps["port"].(map[string]interface{}); ok {
				p["default"] = initial.Port
			}
		}
		if initial.Project != "" {
			if p, ok := flatProps["project"].(map[string]interface{}); ok {
				p["default"] = initial.Project
			}
		}
		if initial.Db != "" {
			if p, ok := flatProps["db"].(map[string]interface{}); ok {
				p["default"] = initial.Db
			}
		}
		if initial.Options != "" {
			if p, ok := flatProps["options"].(map[string]interface{}); ok {
				p["default"] = initial.Options
			}
		}
	}

	reqSchema := schema.ElicitRequestParamsRequestedSchema{Type: "object", Properties: flatProps, Required: required}

	namespace, _ := s.auth.Namespace(ctx)
	messageSuffix := ""
	if !auth.IsDefaultNamespace(namespace) {
		messageSuffix = fmt.Sprintf(" in namespace %s", namespace)
	}

	elicitResult, err := impl.Elicit(ctx, &jsonrpc.TypedRequest[*schema.ElicitRequest]{Request: &schema.ElicitRequest{Params: schema.ElicitRequestParams{
		ElicitationId:   uuid.New().String(),
		Message:         fmt.Sprintf("Please provide connection details%s", messageSuffix),
		RequestedSchema: reqSchema,
	}}})
	if err != nil || elicitResult == nil {
		return "", err
	}
	if elicitResult.Action != schema.ElicitResultActionAccept {
		return "", fmt.Errorf("user: reject adding connection %v", elicitResult.Action)
	}

	// Map content to ConnectionInput.
	var metaInput ConnectionInput
	if err := mapToStruct(elicitResult.Content, &metaInput); err != nil {
		return "", err
	}
	metaCfg := s.matchMeta(metaInput.Driver)
	metaInput.Init(metaCfg)
	if err := metaInput.Validate(metaCfg); err != nil {
		return "", err
	}
	conn := &Connector{Name: metaInput.Name, Driver: metaInput.Driver, DSN: metaInput.Expand(metaCfg.DSN)}
	if _, err := s.Set(ctx, conn); err != nil {
		return "", err
	}
	return conn.Name, nil
}
