package mcp

import (
	"fmt"
	"github.com/viant/mcp-sqlkit/db/connector"
	"github.com/viant/mcp-sqlkit/policy"
	"strings"
)

type Config struct {
	Connector *connector.Config

	// UseData, when set to true, instructs SQLKit to put tool results in the
	// `data` field of CallToolResultContentElem.  When false (default) the
	// result JSON is carried in the `text` field.  This reverses the legacy
	// behaviour where `data` was the default.
	UseData bool `json:"useData,omitempty"`

	// Deprecated: kept for backwards-compatibility with earlier versions that
	// used `useText` (default false).  When both UseText and UseData are set
	// the latter wins.
	UseText bool `json:"useText,omitempty"`
}

func (c *Config) Init(httpAddr string) {
	if c.Connector == nil {
		c.Connector = &connector.Config{}
	}

	// Assign default directory for persisted secrets when not specified.
	if c.Connector.SecretBaseLocation == "" {
		c.Connector.SecretBaseLocation = "~/.secret/mcpt"
	}
	if c.Connector.CallbackBaseURL == "" {
		port := "5000"
		if idx := strings.LastIndex(httpAddr, ":"); idx >= 0 {
			port = httpAddr[idx+1:]
		}
		c.Connector.CallbackBaseURL = fmt.Sprintf("http://localhost:%v", port)
	}
	if c.Connector.Policy == nil {
		c.Connector.Policy = &policy.Policy{}
	}
}
