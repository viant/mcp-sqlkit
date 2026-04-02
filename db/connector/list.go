package connector

import (
	"context"
	"fmt"
	"github.com/viant/mcp-protocol/client"
	"github.com/viant/mcp-protocol/schema"
	"strings"
)

// ListInput represents parameters for the List tool. Currently it is empty but
// defined for forward-compatibility with possible future filters.
type ListInput struct {
	Pattern string `json:"pattern"`
}

// ListOutput represents result returned by the List tool.
type ListOutput struct {
	Data   []interface{} `json:"data,omitempty"`
	Status string        `json:"status"`
	Error  string        `json:"error,omitempty"`
}

// List returns all connectors visible in the caller's namespace.
func (s *Service) List(ctx context.Context) []*Connector {
	namespace, err := s.auth.Namespace(ctx)
	if err != nil || namespace == "" {
		namespace = "default"
	}
	s.logNamespaceConnectors(namespace)
	ns, ok := s.namespace.Get(namespace)
	if !ok {
		return nil
	}
	vals := ns.Connectors.Values()
	return vals
}

// ListConnectors produces ListOutput with all available connectors. It is a
// convenience wrapper used by the MCP toolbox tool registration.
func (s *Service) ListConnectors(ctx context.Context, input *ListInput) *ListOutput {
	output := &ListOutput{Status: "ok"}
	// Use the existing List method.
	connectors := s.List(ctx)

	// If nothing is configured yet, try to ensure a default connection exists,
	// then re-read the list. Prefer a form-based elicitation to collect
	// connection details when the client supports it; fall back to the legacy
	// "dev" name flow otherwise.
	if len(connectors) == 0 {
		if impl, ok := s.mcpClient.(client.Operations); ok && impl.Implements(schema.MethodElicitationCreate) {
			fmt.Printf("[sqlkit-list] no connectors, triggering form elicitation (elicit supported)\n")
			_, _ = s.requestConnectorForm(ctx, impl, &ConnectionInput{})
		} else {
			fmt.Printf("[sqlkit-list] no connectors, elicit NOT supported, falling back to dev\n")
			_, _ = s.Connection(ctx, "dev")
		}
		connectors = s.List(ctx)
	}
	if len(connectors) == 0 {
		// Always return an explicit empty array to avoid omitting the field
		// when marshalled, which some clients interpret as "no data field".
		output.Data = []interface{}{}
		return output
	}

	// Filter connectors based on pattern if provided
	var filteredConnectors []*Connector
	// Be tolerant to nil input (MCP may send null params).
	pattern := ""
	if input != nil {
		pattern = input.Pattern
	}
	if pattern != "" {
		for _, c := range connectors {
			if strings.Contains(c.Name, pattern) {
				filteredConnectors = append(filteredConnectors, c)
			}
		}
	} else {
		filteredConnectors = connectors
	}

	// Return a simplified, serialisation-safe view of connectors to avoid any
	// potential JSON marshalling issues in clients (no internal fields).
	output.Data = make([]interface{}, len(filteredConnectors))
	for i, c := range filteredConnectors {
		output.Data[i] = map[string]interface{}{
			"name":   c.Name,
			"driver": c.Driver,
			"dsn":    c.DSN,
		}
	}
	return output
}
