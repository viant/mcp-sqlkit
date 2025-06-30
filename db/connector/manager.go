package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/viant/jsonrpc"
	"github.com/viant/mcp-protocol/schema"
	"reflect"

	"github.com/viant/mcp-sqlkit/auth"
	"github.com/viant/mcp-sqlkit/db/connector/meta"
	"github.com/viant/scy"
	"github.com/viant/scy/cred"
)

type Manager struct {
	Config     *Config
	metaConfig []*meta.Config
	namespace  *Namespaces
	auth       *auth.Service
	secrets    *scy.Service
	pending    *PendingSecrets
}

func NewManager(cfg *Config, authSvc *auth.Service, secrets *scy.Service) *Manager {
	mgr := &Manager{
		Config:     cfg,
		metaConfig: meta.GetConfigs(),
		namespace:  NewNamespaces(),
		auth:       authSvc,
		secrets:    secrets,
		pending:    NewPendingSecrets(),
	}
	mgr.initDefaultConnectors()
	return mgr
}

// initDefaultConnectors loads connectors supplied via configuration and makes
// them immediately available in their respective namespaces. This function is
// idempotent and should only be called during initial Manager construction.
func (c *Manager) initDefaultConnectors() {
	if c == nil || c.Config == nil || len(c.Config.DefaultConnectors) == 0 {
		return
	}

	for _, nsCfg := range c.Config.DefaultConnectors {
		if nsCfg == nil {
			continue
		}
		nsName := nsCfg.Namespace
		if nsName == "" {
			nsName = "default"
		}

		ns, _ := c.namespace.Get(nsName)
		if ns == nil {
			ns = NewNamespace(nsName)
			c.namespace.Put(nsName, ns)
		}

		for _, conn := range nsCfg.Connectors {
			if conn == nil {
				continue
			}

			// Ensure connectors are linked with the shared secrets service.
			conn.SetSecrets(c.secrets)

			// Persist secret if provided and pointing to local resource but not yet stored.
			if conn.Secrets != nil {
				if _, err := c.secrets.Load(context.Background(), conn.Secrets); err != nil {
					// ignore missing secret – connector will require secret elicitation later.
				}
			}

			ns.Connectors.Put(conn.Name, conn)
		}
	}
}

// Connection retrieves the Connector by name from the caller's namespace. It
// does not perform any UI-specific logic – the Service wrapper is responsible
// for deciding whether to initiate secret elicitation. Typed errors are
// returned so that the caller can differentiate between the failure reasons.
func (c *Manager) Connection(ctx context.Context, name string) (*Connector, error) {
	namespace, err := c.auth.Namespace(ctx)
	if err != nil {
		return nil, err
	}
	ns, ok := c.namespace.Get(namespace)
	if !ok {
		return nil, ErrNamespaceNotFound
	}
	conn, ok := ns.Connectors.Get(name)
	if !ok {
		return nil, ErrConnectorNotFound
	}
	return conn, nil
}

// matchMeta selects meta.Config matching a driver or default one.
func (c *Manager) matchMeta(driver string) *meta.Config {
	for _, metaCfg := range c.metaConfig {
		if metaCfg.Driver == driver {
			return metaCfg
		}
	}
	return &meta.Config{CredType: reflect.TypeOf(cred.Basic{})}
}

// Get returns Connector by name in default namespace (mainly for legacy code
// without context). This replicates previous behaviour but is placed on the
// Manager to avoid duplication.
func (c *Manager) Get(name string) *Connector {
	if c == nil {
		return nil
	}
	ns, ok := c.namespace.Get("default")
	if !ok {
		return nil
	}
	connector, _ := ns.Connectors.Get(name)
	return connector
}

// Pending retrieves pending entry by UUID.
func (c *Manager) Pending(uuid string) (*PendingSecret, bool) {
	if c.pending == nil {
		return nil, false
	}
	return c.pending.Get(uuid)
}

// CompletePending marks pending secret done and activates connector.
func (c *Manager) CompletePending(uuid string) error {
	pend, ok := c.Pending(uuid)
	if !ok {
		return fmt.Errorf("pending secret %s not found", uuid)
	}
	ns, _ := c.namespace.Get(pend.Namespace)
	if ns != nil {
		ns.Connectors.Put(pend.Connector.Name, pend.Connector)
	}
	c.pending.Close(uuid)
	return nil
}

// CancelPending aborts a pending secret submission. Waiting goroutines are
// released but the connector is NOT activated.
func (c *Manager) CancelPending(ctx context.Context, uuid string) error {
	pend, ok := c.Pending(uuid)
	if !ok {
		return fmt.Errorf("pending secret %s not found", uuid)
	}
	// Mark as done to unblock any Add waiters but do NOT add connector.
	c.pending.Close(uuid)
	// Remove from map so it cannot be reused.
	c.pending.Delete(uuid)
	if pend.MCP != nil {
		reason := "cancelled by user"
		requestId, _ := jsonrpc.AsRequestIntId(pend.uiRequest.Id)
		cancelParams := schema.CancelledNotificationParams{
			Reason:    &reason,
			RequestId: schema.RequestId(requestId),
		}
		params, _ := json.Marshal(cancelParams)
		return pend.MCP.Notify(ctx, &jsonrpc.Notification{Method: schema.MethodNotificationCancel, Params: params})
	}
	return nil
}
