package connector

import (
	"github.com/viant/jsonrpc"
	"github.com/viant/mcp-protocol/client"
	"github.com/viant/mcp-protocol/schema"
	"github.com/viant/mcp-protocol/syncmap"
	"github.com/viant/mcp-sqlkit/db/connector/meta"
	"golang.org/x/oauth2"
	"reflect"
)

// PendingSecret represents a connector awaiting credential submission through
// the secret-elicitation flow.
type PendingSecret struct {
	UUID          string
	Namespace     string
	ConnectorMeta *meta.Config
	Connector     *Connector
	NS            *Namespace
	uiRequest     *jsonrpc.TypedRequest[*schema.ElicitRequest]
	CallbackURL   string
	CredType      reflect.Type
	MCP           client.Operations
	OAuth2Config  *oauth2.Config
	done          chan struct{}
}

// PendingSecrets is a concurrency-safe collection of pending entries.
type PendingSecrets struct {
	*syncmap.Map[string, *PendingSecret]
}

func NewPendingSecrets() *PendingSecrets {
	return &PendingSecrets{syncmap.NewMap[string, *PendingSecret]()}
}

// Close signals completion to waiting goroutines.
func (p *PendingSecrets) Close(uuid string) {
	if entry, ok := p.Get(uuid); ok && entry != nil {
		select {
		case <-entry.done:
		default:
			close(entry.done)
		}
	}
}
