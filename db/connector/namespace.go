package connector

import "github.com/viant/mcp-protocol/syncmap"

type (
	Namespace struct {
		Name string
		*Connectors
	}

	Namespaces struct {
		*syncmap.Map[string, *Namespace]
	}

	Namespaced struct {
		Namespace  string
		Connectors []*Connector
	}
)

// NewNamespace namespaces
func NewNamespace(name string) *Namespace {
	return &Namespace{Name: name, Connectors: NewConnectors()}
}

func NewNamespaces() *Namespaces {
	return &Namespaces{syncmap.NewMap[string, *Namespace]()}
}
