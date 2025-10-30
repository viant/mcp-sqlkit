package main

// Options defines CLI flags for the mcp-sqlkit server.
type Options struct {
	HTTPAddr   string `short:"a" long:"addr"  description:"HTTP listen address (empty disables HTTP)"`
	Stdio      bool   `short:"s" long:"stdio" description:"Enable stdio transport"`
	ConfigPath string `short:"c" long:"config" description:"Path to JSON configuration file"`

	// Return tool results using the `data` field instead of the default
	// `text` field (negates the config's default behaviour).
	UseData      bool   `short:"d" long:"data" description:"Return tool results using the 'data' field of CallToolResultContentElem (default uses 'text')"`
	Oauth2Config string `short:"o" long:"oauth2config" description:"Path to JSON OAuth2 configuration file"`
	UserIdToken  bool   `short:"i"  long:"idToken" description:"flag to use id token"`

	// Base URL for secrets storage (scy). Supports mem://, file://,
	// Defaults to in-memory AFS storage.
	SecretBaseLocation string `long:"secretsBase" description:"Base URL for secrets storage (mem://, file://, gcp://secretmanager/projects/xxxx/   ... see for list of secure connector  https://github.com/viant/afsc	)" default:"mem://localhost/mcp-sqlkit/.secret/"`
}
