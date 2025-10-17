package connector

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/viant/jsonrpc"
	"github.com/viant/mcp-protocol/client"
	"github.com/viant/mcp-protocol/schema"
	"github.com/viant/mcp-sqlkit/auth"
	"github.com/viant/mcp-sqlkit/db/connector/meta"
	"github.com/viant/structology/conv"
)

// ConnectionInput is the structure the user supplies when adding a new connector. It
// purposefully omits sensitive data like secrets.
type ConnectionInput struct {
	Name    string `json:"name" description:"Connector name"`
	Driver  string `json:"driver" description:"Connector driver" choice:"mysql" choice:"bigquery" choice:"postgres" choice:"oracle" choice:"sqlite"  `
	Host    string `json:"host,omitempty" description:"Host"`
	Port    int    `json:"port,omitempty" description:"Port"`
	Project string `json:"project,omitempty" description:"Project"`
	Db      string `json:"db,omitempty" description:"DB/Dataset"`
	Options string `json:"options,omitempty" description:"Options"`
}

func (i *ConnectionInput) Init(config *meta.Config) {
	if config == nil {
		return
	}
	if i.Host == "" {
		i.Host = config.Defaults.Host
	}
	if i.Port == 0 {
		i.Port = config.Defaults.Port
	}
	if i.Options == "" {
		i.Options = config.Defaults.Options
	}
}

func (i *ConnectionInput) Validate(config *meta.Config) error {
	if config == nil {
		return fmt.Errorf("invalid driver")
	}
	if i.Host == "" && strings.Contains(config.DSN, "${Host})") {
		return fmt.Errorf("host cannot be empty")
	}
	if i.Port == 0 && strings.Contains(config.DSN, "${Port})") {
		return fmt.Errorf("port cannot be empty")
	}
	return nil
}

func (i *ConnectionInput) Expand(dsn string) string {
	if strings.Contains(dsn, "${Host}") {
		dsn = strings.Replace(dsn, "${Host}", i.Host, 1)
	}
	if strings.Contains(dsn, "${Port}") {
		dsn = strings.Replace(dsn, "${Port}", strconv.Itoa(i.Port), 1)
	}
	if strings.Contains(dsn, "${Options}") {
		dsn = strings.Replace(dsn, "${Options}", i.Options, 1)
	}
	if strings.Contains(dsn, "${Project}") {
		dsn = strings.Replace(dsn, "${Project}", i.Project, 1)
	}
	if strings.Contains(dsn, "${Db}") {
		dsn = strings.Replace(dsn, "${Db}", i.Db, 1)
	}
	return dsn
}

// Get returns an *Connector by name for the current namespace-less context.
// It performs a best-effort lookup ignoring namespace ownership (used mainly by
// legacy callers that did not pass context). Prefer Connection(ctx,name).

// mapToStruct fills a struct pointer with values from a map using JSON marshal/unmarshal round-trip.
func mapToStruct(m map[string]interface{}, out interface{}) error {
	// Leverage structology's converter to map a generic map into the provided struct pointer.
	// Using the default options ensures JSON tag names are respected while keeping
	// case-insensitive matching consistent with the previous marshal/unmarshal behaviour.
	return conv.NewConverter(conv.DefaultOptions()).Convert(m, out)
}

// requestConnectorElicit sends an Elicit request to the client asking for
// connector meta input. It silently ignores errors so that the original call
// can still return a meaningful error to its caller.
func (s *Service) requestConnectorElicit(ctx context.Context, impl client.Operations, connectorName, namespace string) (string, error) {
	// Build JSON schema for ConnectionInput.
	props, required := schema.StructToProperties(reflect.TypeOf(ConnectionInput{}))
	flatProps := make(map[string]interface{}, len(props))
	for k, v := range props {
		flatProps[k] = v
	}
	if connectorName != "" {
		props["name"]["default"] = connectorName
	}

	reqSchema := schema.ElicitRequestParamsRequestedSchema{
		Type:       "object",
		Properties: flatProps,
		Required:   required,
	}
	messageSuffix := ""
	if !auth.IsDefaultNamespace(namespace) {
		messageSuffix = fmt.Sprintf(" in namespace %s", namespace)
	}

	elicitResult, err := impl.Elicit(ctx, &jsonrpc.TypedRequest[*schema.ElicitRequest]{Request: &schema.ElicitRequest{
		Params: schema.ElicitRequestParams{
			ElicitationId:   uuid.New().String(),
			Message:         fmt.Sprintf("Please provide connection details for %s %s", connectorName, messageSuffix),
			RequestedSchema: reqSchema,
		}}})

	if err != nil || elicitResult == nil {
		return connectorName, err
	}
	if elicitResult.Action != schema.ElicitResultActionAccept {
		return connectorName, fmt.Errorf("user: reject adding connection %v", elicitResult.Action)
	}

	// Map result content to ConnectionInput struct
	var metaInput ConnectionInput
	if err := mapToStruct(elicitResult.Content, &metaInput); err != nil {
		return connectorName, err
	}
	metaConfig := s.matchMeta(metaInput.Driver)
	metaInput.Init(metaConfig)
	if err := metaInput.Validate(metaConfig); err != nil {
		return connectorName, err
	}
	conn := &Connector{
		Name:   metaInput.Name,
		Driver: metaInput.Driver,
		DSN:    metaInput.Expand(metaConfig.DSN),
	}
	if _, err := s.Set(ctx, conn); err != nil {
		return "", err
	}
	return conn.Name, nil
}
