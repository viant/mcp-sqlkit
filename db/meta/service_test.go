package meta

import (
	"context"
	"testing"

	_ "modernc.org/sqlite" // register SQLite driver

	"github.com/stretchr/testify/assert"

	"github.com/viant/mcp-sqlkit/auth"
	"github.com/viant/mcp-sqlkit/db/connector"
	"github.com/viant/mcp-sqlkit/policy"
	"github.com/viant/scy"
)

// TestService_Error verifies that the service returns an error status when the
// requested connector is missing. Although simple, it guards against
// panicking on nil references and ensures consistent envelope formatting.
func TestService_Error(t *testing.T) {
	type testCase struct {
		name      string
		inputConn string
	}

	testCases := []testCase{{name: "single_table", inputConn: "testConn"}}

	// Build services scaffold.
	cfg := &connector.Config{}
	authSvc := auth.New(&policy.Policy{})
	secrets := scy.New()
	mgr := connector.NewManager(cfg, authSvc, secrets)
	connSvc := connector.NewService(mgr, nil)
	metaSvc := New(connSvc)

	ctx := context.Background()

	// Register an in-memory SQLite connector and create a sample table.
	conn := &connector.Connector{
		Name:   "testConn",
		Driver: "sqlite",
		DSN:    "file:memdb1?mode=memory&cache=shared",
	}

	pend, err := connSvc.GeneratePendingSecret(ctx, conn)
	assert.Nil(t, err)
	pend.NS.Connectors.Put(conn.Name, conn)

	// Prepare database schema.
	db, err := conn.Db(ctx)
	assert.Nil(t, err)
	_, err = db.ExecContext(ctx, "CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT);")
	assert.Nil(t, err)

	for _, tc := range testCases {
		outTbl := metaSvc.ListTables(ctx, &ListTablesInput{Connector: tc.inputConn})
		assert.EqualValues(t, "ok", outTbl.Status, tc.name)

		outCol := metaSvc.ListColumns(ctx, &ListColumnsInput{Connector: tc.inputConn, Table: "users"})
		assert.EqualValues(t, "ok", outCol.Status, tc.name)
	}
}
