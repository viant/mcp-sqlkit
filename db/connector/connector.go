package connector

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/viant/mcp-protocol/syncmap"
	"github.com/viant/scy"
	"github.com/viant/scy/cred"
)

type Connectors struct {
	*syncmap.Map[string, *Connector]
}

func NewConnectors() *Connectors {
	return &Connectors{syncmap.NewMap[string, *Connector]()}
}

type Connector struct {
	Name        string        `json:"name" yaml:"name"`
	DSN         string        `json:"dsn" yaml:"dsn"`
	Driver      string        `json:"driver" yaml:"driver"`
	Secrets     *scy.Resource `json:"secrets,omitempty" yaml:"secrets,omitempty" internal:"true"`
	db          *sql.DB       `internal:"true"`
	mux         sync.RWMutex  `internal:"true"`
	initialized uint32        `internal:"true"`
	secrets     *scy.Service
}

func (c *Connector) SetSecrets(secrets *scy.Service) {
	c.secrets = secrets
}
func (c *Connector) ExpandDSN(ctx context.Context) (string, error) {
	if !strings.Contains(c.DSN, "$") {
		return c.DSN, nil
	}
	if c.Secrets != nil {
		resource := *c.Secrets
		resource.SetTarget(reflect.TypeOf(&cred.Basic{}))
		if c.secrets == nil {
			c.secrets = scy.New()
		}
		secret, err := c.secrets.Load(ctx, &resource)
		if err != nil {
			return "", err
		}
		return secret.Expand(c.DSN), nil
	}
	return c.DSN, nil
}

func (c *Connector) Close() error {
	c.mux.Lock()
	defer c.mux.Unlock()
	if c.db != nil {
		_ = c.db.Close()
		c.db = nil
	}
	atomic.StoreUint32(&c.initialized, 0)
	return nil
}

func (c *Connector) Db(ctx context.Context) (*sql.DB, error) {
	c.mux.RLock()
	db := c.db
	c.mux.RUnlock()
	if db != nil {
		return db, nil
	}
	c.mux.Lock()
	defer c.mux.Unlock()
	if c.db != nil {
		return c.db, nil
	}
	dsn, _ := c.ExpandDSN(ctx)
	db, err := sql.Open(c.Driver, dsn)
	if err != nil {
		return nil, err
	}
	if atomic.CompareAndSwapUint32(&c.initialized, 0, 1) {
		go c.ensureActive()
	}
	c.db = db
	return db, nil
}

func (c *Connector) ensureActive() {
	for ; atomic.LoadUint32(&c.initialized) == 1; time.Sleep(2 * time.Second) {
	}
	{
		err := c.db.Ping()
		if err != nil {
			c.mux.Lock()
			atomic.StoreUint32(&c.initialized, 0)
			c.db = nil
			c.mux.Unlock()
			return
		}
	}
}
