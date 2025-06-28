package connector

import (
	"context"
	"fmt"
	"github.com/viant/jsonrpc"
	"net/url"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/viant/mcp-protocol/client"
	"github.com/viant/mcp-protocol/schema"
	"github.com/viant/scy"
	"github.com/viant/scy/cred"
	"os"
	"path/filepath"
	"strings"
)

// Add registers or updates a connector in the caller's namespace.  If its secret
// already exists, the connector becomes ACTIVE immediately.  Otherwise it is
// placed in PENDING_SECRET state and, provided the client supports
// CreateUserInteraction, a browser flow is initiated to collect the secret
// value.  The method never returns the secret and therefore is safe over MCP
// RPC.
func (s *Service) Add(ctx context.Context, connector *Connector) error {
	pend, err := s.GeneratePendingSecret(ctx, connector)
	if err != nil {
		return err
	}
	pend.MCP = s.mcpClient
	connector.secrets = s.secrets
	// If client can handle CreateUserInteraction generate it and optionally wait.
	if impl, ok := s.mcpClient.(client.Operations); ok && impl.Implements(schema.MethodElicitationCreate) {
		_, _ = impl.Elicit(ctx, &jsonrpc.TypedRequest[*schema.ElicitRequest]{
			Request: &schema.ElicitRequest{
				Params: schema.ElicitRequestParams{
					Message: "Initiate secrets flow for " + connector.Name + " connector",
					RequestedSchema: schema.ElicitRequestParamsRequestedSchema{
						Properties: map[string]interface{}{
							"flowURI": map[string]interface{}{
								"default":     pend.CallbackURL,
								"type":        "string",
								"title":       "Flow URI",
								"description": "URI of the flow to initiate",
							},
						},
						Required: []string{"flowURI"},
					},
				}}})

		// Wait for secret submission up to 5 min.
		select {
		case <-pend.done:
			if connector.Secrets != nil {
				res := *connector.Secrets
				res.SetTarget(pend.CredType)
				if _, err := s.secrets.Load(ctx, &res); err == nil {
					pend.NS.Connectors.Put(connector.Name, connector)
				}
			} else {
				pend.NS.Connectors.Put(connector.Name, connector)
			}
		case <-time.After(5 * time.Minute):
		case <-ctx.Done():
		}
	}
	return nil
}

func (s *Service) GeneratePendingSecret(ctx context.Context, connector *Connector) (*PendingSecret, error) {
	namespace, err := s.auth.Namespace(ctx)
	if err != nil {
		return nil, err
	}
	ns, ok := s.namespace.Get(namespace)
	if !ok {
		ns = NewNamespace(namespace)
		s.namespace.Put(namespace, ns)
	}

	// Determine credential type for this driver.
	metaCfg := s.matchMeta(connector.Driver)
	credType := metaCfg.CredType
	if credType == nil || credType.Kind() == reflect.Invalid {
		credType = reflect.TypeOf(cred.Basic{})
	}

	encodedNS := url.QueryEscape(namespace)

	// Determine resource URL for secret storage.  When SecretBaseLocation is
	// provided, store the secret on the local filesystem under the following
	// layout:
	//   <base>/<driver>/<dbname>/<namespace>
	// Otherwise fall back to in-memory storage.
	var resURL string
	if base := s.Config.SecretBaseLocation; base != "" {
		// Expand a leading ~ to the user's home directory so that the default
		// value works cross-platform without additional configuration.
		if strings.HasPrefix(base, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				base = filepath.Join(home, base[2:])
			}
		}

		// Attempt to extract database name from DSN so that the path includes
		// driver, dbname and namespace as requested.
		dbName := extractDBName(connector.DSN)

		fullPath := filepath.Join(base, connector.Driver, dbName, encodedNS)
		// Convert to URI – scy expects a file:// scheme for filesystem secrets.
		resURL = fmt.Sprintf("file://%s", filepath.ToSlash(fullPath))
	} else {
		// Legacy behaviour – keep secret in memory.
		resURL = fmt.Sprintf("mem://localhost/%s/%s", connector.Name, encodedNS)
	}

	if metaCfg.CredType == reflect.TypeOf(&cred.Basic{}) {
		connector.Secrets = scy.NewResource("", resURL, "blowfish://default")
	}

	pend := &PendingSecret{
		UUID:          uuid.NewString(),
		Namespace:     namespace,
		NS:            ns,
		ConnectorMeta: metaCfg,
		Connector:     connector,
		CredType:      credType,
		done:          make(chan struct{}),
	}

	// Build callback URL (simplified for now).
	baseURL := "http://localhost"
	if s.Config != nil && s.Config.CallbackBaseURL != "" {
		baseURL = strings.TrimRight(s.Config.CallbackBaseURL, "/")
	}
	pend.CallbackURL = fmt.Sprintf("%s/ui/interaction/%s", baseURL, pend.UUID)
	s.pending.Put(pend.UUID, pend)
	return pend, nil
}

// extractDBName attempts to derive the database name from the DSN string.
// It supports common DSN formats (MySQL "user:pass@tcp(host)/dbname?params"
// or URL style "postgres://user:pass@host/dbname?params"). When the database
// name cannot be determined, the connector name is returned as a fallback to
// maintain backward-compatible uniqueness.
func extractDBName(dsn string) string {
	if dsn == "" {
		return "default"
	}
	// Trim trailing parameters if present.
	trimmed := dsn
	if idx := strings.Index(trimmed, "?"); idx != -1 {
		trimmed = trimmed[:idx]
	}
	// Remove a trailing slash if DSN ends with it.
	trimmed = strings.TrimRight(trimmed, "/")
	if idx := strings.LastIndex(trimmed, "/"); idx != -1 && idx+1 < len(trimmed) {
		return trimmed[idx+1:]
	}
	return "default"
}
