// Package driver registers a set of commonly used database/sql drivers as well
// as metadata product definitions from github.com/viant/sqlx. Importing this
// package (with a blank identifier) guarantees that both the driver `sql.Open`
// calls and sqlx/metadata product detection work out-of-the-box.
//
//	import _ "github.com/viant/mcp-sqlkit/db/driver"
//
// The package has **no public API** – its only purpose is to execute `init`
// side-effects of the imported modules.
package driver

import (
	// database/sql drivers
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	// Viant drivers
	_ "github.com/sijms/go-ora/v2"
	_ "github.com/viant/bigquery"

	// sqlx metadata product registrations (ensure dialect specific queries are available)
	_ "github.com/viant/sqlx/metadata/product/ansi"
	_ "github.com/viant/sqlx/metadata/product/bigquery"
	_ "github.com/viant/sqlx/metadata/product/mysql"
	_ "github.com/viant/sqlx/metadata/product/oracle"
	_ "github.com/viant/sqlx/metadata/product/pg"
	_ "github.com/viant/sqlx/metadata/product/sqlite"
)
