package meta

import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/viant/scy/cred"
	_ "modernc.org/sqlite"
	"reflect"

	_ "github.com/viant/aerospike"
	_ "github.com/viant/bigquery"
	_ "github.com/viant/firebase/firestore"
	_ "github.com/viant/firebase/realtime"
)

type (
	Config struct {
		Driver       string
		DSN          string
		Defaults     Defaults
		Oauth2Config *cred.Oauth2Config
		CredType     reflect.Type
	}

	Defaults struct {
		Port    int
		Host    string
		Options string
		Scopes  []string
	}
)

// GetConfigs returns built-in driver metadata used by the connector service.
func GetConfigs() []*Config {
	return []*Config{
		{
			Driver:   "mysql",
			DSN:      "$Username:$Password@tcp(${Host}:${Port})/${Db}?${Options}",
			CredType: reflect.TypeOf(&cred.Basic{}),
			Defaults: Defaults{
				Port:    3306,
				Options: "parseTime=true",
				Host:    "localhost",
			},
		},
		{
			Driver:   "bigquery",
			DSN:      "bigquery://${Project}/${Db}?${Options}",
			CredType: reflect.TypeOf(&cred.Oauth2Config{}),
			Defaults: Defaults{
				Scopes: []string{"https://www.googleapis.com/auth/bigquery",
					"https://www.googleapis.com/auth/userinfo.email",
					"https://www.googleapis.com/auth/cloud-platform"},
			},
		},
		{
			Driver:   "postgres",
			DSN:      "postgres://$Username:$Password@${Host}:${Port}/${Db}?${Options}",
			CredType: reflect.TypeOf(&cred.Basic{}),
			Defaults: Defaults{
				Port:    5432,
				Options: "sslmode=disable",
				Host:    "localhost",
			},
		},
	}
}
