package connector

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"time"

	"github.com/viant/jsonrpc"

	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/viant/mcp-protocol/client"
	"github.com/viant/mcp-protocol/schema"
	"github.com/viant/scy"
	"github.com/viant/scy/cred"
)

// Set registers or updates a connector in the caller's namespace.  If its secret
// already exists, the connector becomes ACTIVE immediately.  Otherwise it is
// placed in PENDING_SECRET state and, provided the client supports the
// MCP Elicit protocol, a browser flow is initiated to collect the secret
// value.  The method never returns the secret and therefore is safe over MCP
// RPC.
func (s *Service) Set(ctx context.Context, connector *Connector) (*AddOutput, error) {
	return s.set(ctx, connector, "")
}

func (s *Service) set(ctx context.Context, connector *Connector, userName string) (*AddOutput, error) {
	pend, err := s.GeneratePendingSecret(ctx, connector)
	if err != nil {
		return nil, err
	}
	pend.UserName = userName
	pend.MCP = s.mcpClient
	connector.secrets = s.secrets
	pend.NS.Connectors.Put(connector.Name, connector)

	if s.secretExists(ctx, connector, pend.CredType) {
		s.pending.Delete(pend.UUID)
		return &AddOutput{Status: "ok", Connector: connector.Name}, nil
	}

	// If client can handle the Elicit protocol generate it and optionally wait.
	if impl, ok := s.mcpClient.(client.Operations); ok && impl.Implements(schema.MethodElicitationCreate) {
		elicitID := uuid.New().String()
		pend.ElicitID = elicitID
		oobURL := pend.CallbackURL
		if strings.Contains(oobURL, "?") {
			oobURL += "&elicitationId=" + url.QueryEscape(elicitID)
		} else {
			oobURL += "?elicitationId=" + url.QueryEscape(elicitID)
		}
		fmt.Printf("[sqlkit-elicit-oob] elicitID=%s connector=%s oobURL=%s\n", elicitID, connector.Name, oobURL)
		elicitResult, _ := impl.Elicit(ctx, &jsonrpc.TypedRequest[*schema.ElicitRequest]{
			Request: &schema.ElicitRequest{
				Params: schema.ElicitRequestParams{
					ElicitationId: elicitID,
					Message:       "Open URL to provide secrets for " + connector.Name + " connector",
					Mode:          "oob",
					Url:           oobURL,
				}}})
		fmt.Printf("[sqlkit-elicit-oob] result=%v\n", elicitResult)

		if elicitResult != nil {
			if elicitResult.Action != schema.ElicitResultActionAccept {
				return nil, fmt.Errorf("user reject providing credentials %v", err)
			}
		}
		// Wait for secret submission up to 5 min.
		select {
		case <-pend.done:
			if s.secretExists(ctx, connector, pend.CredType) {
				pend.NS.Connectors.Put(connector.Name, connector)
			}
		case <-time.After(5 * time.Minute):
			// Timed out – return pending state with callback URL
			return &AddOutput{Status: "ok", State: "PENDING_SECRET", CallbackURL: pend.CallbackURL, Connector: connector.Name}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		// Secret submitted within wait window – connector activated
		return &AddOutput{Status: "ok", Connector: connector.Name}, nil
	}
	// Client cannot handle Elicit – the connector remains pending waiting for
	// secret to be supplied out-of-band; return pending state with callback URL.
	return &AddOutput{Status: "ok", State: "PENDING_SECRET", CallbackURL: pend.CallbackURL, Connector: connector.Name}, nil
}

func (s *Service) GeneratePendingSecret(ctx context.Context, connector *Connector) (*PendingSecret, error) {
	namespace, err := s.auth.Namespace(ctx)
	if err != nil || namespace == "" {
		namespace = "default"
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

	if metaCfg.CredType == reflect.TypeOf(&cred.Basic{}) && (connector.Secrets == nil || connector.Secrets.URL == "") {
		connector.Secrets = scy.NewResource("", s.defaultSecretURL(namespace, connector), "blowfish://default")
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

func (s *Service) secretExists(ctx context.Context, connector *Connector, credType reflect.Type) bool {
	if connector == nil || connector.Secrets == nil || s.secrets == nil {
		return false
	}
	resource := *connector.Secrets
	if err := normalizeSecretResourceURL(&resource); err != nil {
		return false
	}
	if credType != nil {
		resource.SetTarget(credType)
	}
	_, err := s.secrets.Load(ctx, &resource)
	return err == nil
}

func (s *Service) defaultSecretURL(namespace string, connector *Connector) string {
	base := s.Config.SecretBaseLocation
	if base == "" {
		base = "mem://localhost/mcp-sqlkit/.secret/"
	}
	encodedNS := url.PathEscape(namespace)
	connectorName := "default"
	driver := "unknown"
	if connector != nil {
		if connector.Name != "" {
			connectorName = url.PathEscape(connector.Name)
		}
		if connector.Driver != "" {
			driver = connector.Driver
		}
	}
	if strings.HasPrefix(base, "file://") {
		fsBase := strings.TrimPrefix(base, "file://")
		if strings.HasPrefix(fsBase, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				fsBase = filepath.Join(home, fsBase[2:])
			}
		}
		fullPath := filepath.Join(fsBase, driver, encodedNS, connectorName)
		return fmt.Sprintf("file://%s", filepath.ToSlash(fullPath))
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return fmt.Sprintf("%s%s/%s/%s.json", base, driver, encodedNS, connectorName)
}
