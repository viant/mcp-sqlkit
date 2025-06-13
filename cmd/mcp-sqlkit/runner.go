package main

// This file contains application bootstrap logic for the mcp-sqlkit command.
// The previous implementation of run() had over 180 lines and mixed many
// responsibilities. It has been broken down into focused helpers while keeping
// the observable behaviour intact.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"reflect"
	"syscall"
	"time"

	"github.com/jessevdk/go-flags"
	"github.com/viant/mcp-protocol/authorization"
	"github.com/viant/mcp-protocol/oauth2/meta"
	"github.com/viant/mcp-protocol/schema"
	"github.com/viant/mcp-sqlkit/db/connector"
	"github.com/viant/mcp-sqlkit/mcp"
	"github.com/viant/mcp-sqlkit/policy"
	mcpsrv "github.com/viant/mcp/server"
	serverauth "github.com/viant/mcp/server/auth"
	"github.com/viant/scy"
	"github.com/viant/scy/auth/flow"
	"github.com/viant/scy/cred"
)

// run is invoked by main and orchestrates CLI parsing, configuration loading,
// server construction and graceful shutdown. Helpers below keep each concern
// isolated and test-friendly.
func run(argv []string) error {
	// 1. Parse CLI flags ----------------------------------------------------
	opts, err := parseFlags(argv)
	if err != nil {
		return err
	}

	// 2. Load configuration (file or sensible defaults) --------------------
	cfg, err := loadConfig(opts)
	if err != nil {
		return err
	}
	cfg.Init(opts.HTTPAddr) // expand runtime defaults

	// 3. Create toolbox service & handler ----------------------------------
	service := mcp.NewService(cfg)
	srvOpts := append(coreOptions(service), oauthOptions(cfg)...)

	srv, err := mcpsrv.New(srvOpts...)
	if err != nil {
		return fmt.Errorf("failed to build server: %w", err)
	}

	// 4. Start transports ---------------------------------------------------
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpSrv := startHTTP(ctx, srv, opts.HTTPAddr)
	stdioCh := startStdio(ctx, srv, opts.Stdio)

	// 5. Wait for termination ----------------------------------------------
	if err := waitForShutdown(ctx, stdioCh); err != nil {
		return err
	}
	return gracefulShutdown(httpSrv)
}

// -------------------------------------------------------------------------
// Helpers

func parseFlags(args []string) (*Options, error) {
	opts := &Options{}
	_, err := flags.ParseArgs(opts, args)
	if err == nil {
		return opts, nil
	}
	// flags returns *flags.Error for help – treat as non error.
	var fe *flags.Error
	if errors.As(err, &fe) && fe.Type == flags.ErrHelp {
		return nil, nil
	}
	return nil, err
}

// loadConfig reads JSON config (when provided) or assembles a sensible default
// that uses an OAuth2 secret from $HOME/.secret/idp_viant.enc .
func loadConfig(opts *Options) (*mcp.Config, error) {
	if opts == nil {
		return &mcp.Config{}, nil
	}

	if opts.ConfigPath != "" {
		data, err := os.ReadFile(opts.ConfigPath)
		if err != nil {
			return nil, err
		}
		var cfg *mcp.Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
    if opts.UseData {
        cfg.UseData = true // CLI override
		}
		return cfg, nil
	}

	// Build implicit default config ---------------------------------------
	sec := scy.New()
	resPath := path.Join(os.Getenv("HOME"), ".secret/idp_viant.enc")
	resource := scy.NewResource(reflect.TypeOf(&cred.Oauth2Config{}), resPath, "blowfish://default")
	secret, err := sec.Load(context.Background(), resource)
	if err != nil {
		return nil, fmt.Errorf("unable to load default OAuth2 secret: %w", err)
	}
	oauthCfg := secret.Target.(*cred.Oauth2Config)

	cfg := &mcp.Config{
		Connector: &connector.Config{
			Policy: &policy.Policy{
				RequireIdentityToken: true,
				Oauth2Config:         &oauthCfg.Config,
			},
		},
    UseData: opts.UseData,
	}
	return cfg, nil
}

// coreOptions returns server options that are always enabled.
func coreOptions(service *mcp.Service) []mcpsrv.Option {
	return []mcpsrv.Option{
		mcpsrv.WithNewHandler(mcp.NewHandler(service)),
		mcpsrv.WithCustomHTTPHandler("/ui/interaction/", service.UI().Handle),
		mcpsrv.WithImplementation(schema.Implementation{Name: "mcp-sqlkit", Version: "1.0"}),
	}
}

// oauthOptions conditionally builds auth-related server options.
func oauthOptions(cfg *mcp.Config) []mcpsrv.Option {
	if cfg == nil || cfg.Connector == nil || cfg.Connector.Policy == nil || cfg.Connector.Policy.Oauth2Config == nil {
		return nil
	}

	authPolicy := &authorization.Policy{
		Global: &authorization.Authorization{
			UseIdToken: cfg.Connector.Policy.RequireIdentityToken,
			ProtectedResourceMetadata: &meta.ProtectedResourceMetadata{
				AuthorizationServers: []string{cfg.Connector.Policy.Oauth2Config.Endpoint.AuthURL},
			},
		},
		ExcludeURI: "/sse", // SSE stream stays unauthenticated
	}

	bff := cfg.Connector.BackendForFrontend
	if bff == nil {
		bff = &serverauth.BackendForFrontend{}
	}
	bff.Client = cfg.Connector.Policy.Oauth2Config
	if bff.AuthorizationExchangeHeader == "" {
		bff.AuthorizationExchangeHeader = flow.AuthorizationExchangeHeader
	}
	authSvc, err := serverauth.New(&serverauth.Config{Policy: authPolicy, BackendForFrontend: bff})
	if err != nil {
		log.Printf("warning: failed to initialise auth service – running without OAuth: %v", err)
		return nil
	}
	return []mcpsrv.Option{
		mcpsrv.WithAuthorizer(authSvc.Middleware),
		mcpsrv.WithProtectedResourcesHandler(authSvc.ProtectedResourcesHandler),
	}
}

// startHTTP boots the HTTP transport when addr is non-empty.
func startHTTP(ctx context.Context, srv *mcpsrv.Server, addr string) *http.Server {
	if addr == "" {
		return nil
	}
	httpSrv := srv.HTTP(ctx, addr)
	go func() {
		log.Printf("mcp-sqlkit listening on HTTP %s", addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()
	return httpSrv
}

// startStdio boots the stdio transport if enabled.
func startStdio(ctx context.Context, srv *mcpsrv.Server, enabled bool) <-chan error {
	if !enabled {
		return nil
	}
	ch := make(chan error, 1)
	go func() {
		log.Printf("mcp-sqlkit listening on stdio")
		ch <- srv.Stdio(ctx).ListenAndServe()
	}()
	return ch
}

// waitForShutdown blocks until CTRL-C or stdio transport terminates.
func waitForShutdown(ctx context.Context, stdio <-chan error) error {
	select {
	case <-ctx.Done():
		log.Printf("shutting down…")
		return nil
	case err := <-stdio:
		if err != nil {
			return fmt.Errorf("stdio server terminated: %w", err)
		}
	}
	return nil
}

// gracefulShutdown attempts to close HTTP server within 5s.
func gracefulShutdown(srv *http.Server) error {
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
