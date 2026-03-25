package main

// Options defines CLI flags for the mcp-sqlkit server.
type Options struct {
	HTTPAddr              string `short:"a" long:"addr"  description:"HTTP listen address (empty disables HTTP)"`
	Stdio                 bool   `short:"s" long:"stdio" description:"Enable stdio transport"`
	ConfigPath            string `short:"c" long:"config" description:"Path to JSON configuration file"`
	DefaultConnectorsPath string `short:"d" long:"default-connectors" description:"Path to JSON file containing default connectors only (either an array of namespaced connector entries or an object with connector.defaultConnectors)"`

	// Return tool results using the `data` field instead of the default
	// `text` field (negates the config's default behaviour).
	UseData      bool   `long:"data" description:"Return tool results using the 'data' field of CallToolResultContentElem (default uses 'text')"`
	Oauth2Config string `short:"o" long:"oauth2config" description:"Path to JSON OAuth2 configuration file"`
	UserIdToken  bool   `short:"i"  long:"idToken" description:"flag to use id token"`

	// Public base URL used for OOB flows and callbacks, e.g.
	//   http://mcp-sqlkit.agently.svc.cluster.local:7789
	// When provided, overrides any derived localhost base.
	PublicBaseURL string `long:"public-base-url" description:"Public base URL for OOB callbacks (e.g. http://mcp-sqlkit.agently.svc.cluster.local:7789)"`

	// Base URL for secrets storage (scy). Supports mem://, file://,
	// Defaults to in-memory AFS storage.
	SecretBaseLocation string `long:"secretsBase" description:"Base URL for secrets storage (mem://, file://, gcp://secretmanager/projects/xxxx/   ... see for list of secure connector  https://github.com/viant/afsc	)" default:"mem://localhost/mcp-sqlkit/.secret/"`
}
