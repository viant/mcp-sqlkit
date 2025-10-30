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

	// SecretBaseLocation specifies the base URL where connection secrets should
	// be stored. It can be any scheme supported by scy (e.g. mem://, file://,
	// gsecret://, vault://, ...). The default is an in-memory AFS path:
	//   mem://localhost/mcp-sqlkit/.secret/
	// The final secret location for a connector is constructed as:
	//   <SecretBaseLocation>/<driver>/<dbname>/<namespace>[.json]
	// ensuring secrets are isolated per driver, database name and caller
	// namespace. The optional .json suffix is appended for non-file schemes.
	SecretBaseLocation string
}
