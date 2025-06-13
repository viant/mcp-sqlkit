package connector

import (
	"github.com/viant/mcp-sqlkit/policy"
	"github.com/viant/mcp/server/auth"
)

type Config struct {
	Policy            *policy.Policy
	DefaultConnectors []*Namespaced
	CallbackBaseURL   string

	BackendForFrontend *auth.BackendForFrontend `json:"backendForFrontend,omitempty"  yaml:"backendForFrontend,omitempty"`

	// SecretBaseLocation specifies the base directory where connection secrets
	// should be stored. When left empty, secrets are kept in-memory only. The
	// default value (~/.secret/mcpt) is assigned by mcp.Config.Init().
	//
	// The final secret location for a connector will be constructed as:
	//   <SecretBaseLocation>/<driver>/<dbname>/<namespace>
	// ensuring secrets are isolated per driver, database name and caller
	// namespace.
	SecretBaseLocation string
}
