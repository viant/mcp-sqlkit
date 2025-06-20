package connector

import (
	"context"
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
	if err != nil {
		return nil
	}
	ns, ok := s.namespace.Get(namespace)
	if !ok {
		return nil
	}
	return ns.Connectors.Values()
}

// ListConnectors produces ListOutput with all available connectors. It is a
// convenience wrapper used by the MCP toolbox tool registration.
func (s *Service) ListConnectors(ctx context.Context, input *ListInput) *ListOutput {
	output := &ListOutput{Status: "ok"}
	// Use the existing List method.
	connectors := s.List(ctx)
	if connectors == nil {
		return output
	}

	// Filter connectors based on pattern if provided
	var filteredConnectors []*Connector
	if input.Pattern != "" {
		for _, c := range connectors {
			if strings.Contains(c.Name, input.Pattern) {
				filteredConnectors = append(filteredConnectors, c)
			}
		}
	} else {
		filteredConnectors = connectors
	}

	output.Data = make([]interface{}, len(filteredConnectors))
	for i, c := range filteredConnectors {
		output.Data[i] = c
	}
	return output
}
