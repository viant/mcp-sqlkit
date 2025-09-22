package interaction

import (
	"context"
	_ "embed"
	"fmt"
	_ "github.com/viant/afs/mem"
	"github.com/viant/afs/url"
	"github.com/viant/bigquery"
	"github.com/viant/mcp-sqlkit/db/connector"
	"github.com/viant/scy"
	"github.com/viant/scy/auth/flow"
	"github.com/viant/scy/auth/gcp/client"
	"github.com/viant/scy/cred"
	"github.com/viant/velty"
	"net/http"
	nurl "net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

//go:embed asset/basic_cred.html
var basicCred []byte

//go:embed asset/notify.js
var notifyJS []byte

// Service handles /ui/interaction/{uuid} endpoints.
type Service struct {
	Connector *connector.Manager
	Secrets   *scy.Service
}

// New builds interaction service.
func New(connectors *connector.Manager, secrets *scy.Service) *Service {
	return &Service{Connector: connectors, Secrets: secrets}
}

// Register attaches HTTP handlers to provided mux.
func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("/ui/interaction/", s.Handle)
	mux.HandleFunc("/ui/asset/", s.serveAsset)
}

func (s *Service) Handle(w http.ResponseWriter, r *http.Request) {
	// URL pattern: /ui/interaction/{uuid}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 { // ui interaction uuid
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	uuid := parts[2]

	pend, ok := s.Connector.Pending(uuid)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// If this is a status landing (completed/cancelled/error), render minimal page
	qs := r.URL.Query()
	if st := qs.Get("status"); st != "" && qs.Get("elicitationId") != "" {
		s.renderStatusNotify(w, st, qs.Get("elicitationId"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		err := s.handleGet(w, r, pend)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		s.handlePost(w, r, pend)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Service) basicCredHTML(data map[string]string) ([]byte, error) {
	planner := velty.New()
	for k, v := range data {
		planner.DefineVariable(k, v)
	}
	execution, newState, err := planner.Compile(basicCred)
	if err != nil {
		return nil, err
	}
	state := newState()
	for k, v := range data {
		state.SetValue(k, v)
	}
	if err = execution.Exec(state); err != nil {
		return nil, err
	}
	return state.Buffer.Bytes(), nil
}

func (s *Service) handleGet(w http.ResponseWriter, r *http.Request, pend *connector.PendingSecret) error {
	if pend.Connector.Driver == "bigquery" {
		return s.handleBigQueryConnectionSetup(w, r, pend)
	}

	host, port, db, options := s.extractBasicFields(pend.Connector.Driver, pend.Connector.DSN)
	s.renderBasicCred(w, map[string]string{
		"Dsn":          pend.Connector.DSN,
		"Connector":    pend.Connector.Name,
		"UUID":         pend.UUID,
		"ErrorMessage": "",
		"Name":         pend.Connector.Name,
		"Host":         host,
		"Port":         port,
		"Db":           db,
		"Project":      "",
		"Options":      options,
	})
	return nil
}

func (s *Service) handleBigQueryConnectionSetup(w http.ResponseWriter, r *http.Request, pend *connector.PendingSecret) error {
	err := s.ensureGCPOauth2Client(pend, r)
	if err != nil {
		return err
	}
	query := r.URL.Query()
	redirectURI := url.Join(s.Connector.Config.CallbackBaseURL, "/ui/interaction/", pend.UUID)
	authCode := query.Get("code")
	config := *pend.OAuth2Config
	config.RedirectURL = redirectURI

	if authCode == "" {
		authorizationURI, err := flow.BuildAuthCodeURL(&config, flow.WithRedirectURI(redirectURI), flow.WithScopes(pend.ConnectorMeta.Defaults.Scopes...))
		if err != nil {
			return err
		}
		http.Redirect(w, r, authorizationURI, http.StatusFound)
		return nil
	}

	token, err := config.Exchange(r.Context(), authCode)
	if err != nil {
		return err
	}
	firstSeparator := "?"
	if strings.Contains(r.URL.Path, firstSeparator) {
		firstSeparator = "&"
	}
	mgr := bigquery.NewOAuth2Manager()
	configURL, err := mgr.WithConfigURL(r.Context(), pend.OAuth2Config)
	if err != nil {
		return err
	}
	tokenURI, err := mgr.WithTokenURL(r.Context(), token)
	if err != nil {
		return err
	}
	pend.Connector.DSN += firstSeparator + "oauth2ClientURL=" + nurl.QueryEscape(configURL) + "&oauth2TokenURL=" + nurl.QueryEscape(tokenURI)
	s.handleCompletion(w, r, pend)
	return nil
}

func (s *Service) renderBasicCred(w http.ResponseWriter, data map[string]string) {
	content, err := s.basicCredHTML(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}

func (s *Service) handlePost(w http.ResponseWriter, r *http.Request, pend *connector.PendingSecret) {
	data, err := s.postData(r)
	if err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
	}
	// Allow editing non-secret connection params and rebuild DSN prior to saving secret.
	postedName := data["name"]
	postedHost := data["host"]
	postedPort := data["port"]
	postedDb := data["db"]
	postedProject := data["project"]
	postedOptions := data["options"]

	// Derive current values from the DSN as fallback.
	curHost, curPort, curDb, curOptions := s.extractBasicFields(pend.Connector.Driver, pend.Connector.DSN)
	effName := pend.Connector.Name
	if postedName != "" {
		effName = postedName
	}
	effHost := curHost
	if postedHost != "" {
		effHost = postedHost
	}
	effPort := curPort
	if postedPort != "" {
		effPort = postedPort
	}
	effDb := curDb
	if postedDb != "" {
		effDb = postedDb
	}
	effProject := ""
	if postedProject != "" {
		effProject = postedProject
	}
	effOptions := curOptions
	if postedOptions != "" {
		effOptions = postedOptions
	}

	// Rebuild DSN using the connector metadata template.
	var portVal int
	if effPort != "" {
		if p, err := strconv.Atoi(effPort); err == nil {
			portVal = p
		}
	}
	metaCfg := pend.ConnectorMeta
	metaIn := &connector.ConnectionInput{Name: effName, Driver: pend.Connector.Driver, Host: effHost, Port: portVal, Project: effProject, Db: effDb, Options: effOptions}
	metaIn.Init(metaCfg)
	// Rebuild DSN and apply unconditionally so edited values take effect for
	// the connectivity check. Validation errors (if any) will surface via
	// the connection error path below. Ensure any existing DB handle is
	// reset so changes (like port) are used for the ping.
	newDSN := metaIn.Expand(metaCfg.DSN)
	dsnChanged := (pend.Connector.DSN != newDSN) || (pend.Connector.Name != metaIn.Name)
	pend.Connector.Name = metaIn.Name
	pend.Connector.DSN = newDSN
	if dsnChanged {
		_ = pend.Connector.Close()
	}

	username := data["username"]
	password := data["password"]

	if act, ok := data["action"]; ok && act == "cancel" {
		_ = s.Connector.CancelPending(r.Context(), pend.UUID)
		// Redirect to status landing with parameters for external notify script
		http.Redirect(w, r, "/ui/interaction/"+pend.UUID+"?elicitationId="+nurl.QueryEscape(pend.ElicitID)+"&status=cancelled", http.StatusSeeOther)
		return
	}

	if username == "" {
		s.renderBasicCred(w, map[string]string{
			"Dsn":          pend.Connector.DSN,
			"Connector":    pend.Connector.Name,
			"UUID":         pend.UUID,
			"ErrorMessage": "Username is required",
			"Name":         effName,
			"Host":         effHost,
			"Port":         effPort,
			"Db":           effDb,
			"Project":      effProject,
			"Options":      effOptions,
		})
		return
	}
	if password == "" {
		s.renderBasicCred(w, map[string]string{
			"Dsn":          pend.Connector.DSN,
			"Connector":    pend.Connector.Name,
			"UUID":         pend.UUID,
			"ErrorMessage": "Password is required",
			"Name":         effName,
			"Host":         effHost,
			"Port":         effPort,
			"Db":           effDb,
			"Project":      effProject,
			"Options":      effOptions,
		})
		return
	}

	basicCred := &cred.Basic{
		Username: username,
		Password: password,
	}

	resource := pend.Connector.Secrets
	resource.SetTarget(reflect.TypeOf(basicCred))
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	secret := scy.NewSecret(basicCred, resource)

	err = s.Secrets.Store(r.Context(), secret)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to store secret %v %v", secret.Resource.URL, err), http.StatusInternalServerError)
		return
	}

	db, err := pend.Connector.Db(ctx)
	if err == nil {
		err = db.Ping()
	}
	if err != nil {
		s.renderBasicCred(w, map[string]string{
			"Dsn":          pend.Connector.DSN,
			"Connector":    pend.Connector.Name,
			"UUID":         pend.UUID,
			"ErrorMessage": fmt.Sprintf("failed to connect to database: %v", err),
			"Name":         effName,
			"Host":         effHost,
			"Port":         effPort,
			"Db":           effDb,
			"Project":      effProject,
			"Options":      effOptions,
		})
		return
	}

	s.handleCompletion(w, r, pend)
}

func (s *Service) handleCompletion(w http.ResponseWriter, r *http.Request, pend *connector.PendingSecret) {
	// Mark pending completed.
	_ = s.Connector.CompletePending(pend.UUID)
	// Redirect to status landing (GET) so the external script can auto-notify+close
	http.Redirect(w, r, "/ui/interaction/"+pend.UUID+"?elicitationId="+nurl.QueryEscape(pend.ElicitID)+"&status=completed", http.StatusSeeOther)
}

func (s *Service) postData(r *http.Request) (map[string]string, error) {
	var data = make(map[string]string)
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	for k, v := range r.PostForm {
		if len(v) > 0 {
			data[k] = v[0]
		}
	}
	return data, nil
}

// serveAsset serves small static assets like notify.js from embedded data.
func (s *Service) serveAsset(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/ui/asset/")
	switch path {
	case "notify.js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(notifyJS)
		return
	default:
		http.NotFound(w, r)
	}
}

// renderStatusNotify renders a minimal landing page that loads notify.js which
// reads URL query parameters (elicitationId, status) to notify the opener and close.
func (s *Service) renderStatusNotify(w http.ResponseWriter, status, elicitID string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// We do not inline scripts; notify.js reads query params.
	msg := "Connector status: " + status + ". This tab will close automatically."
	_, _ = w.Write([]byte("<html><body><h3>" + msg + "</h3><script src=\"/ui/asset/notify.js\"></script></body></html>"))
}

// extractBasicFields attempts to parse host, port, database and options from
// DSN strings produced by built-in templates for mysql and postgres drivers.
func (s *Service) extractBasicFields(driver, dsn string) (host, port, db, options string) {
	switch driver {
	case "postgres":
		if u, err := nurl.Parse(dsn); err == nil {
			host = u.Hostname()
			port = u.Port()
			db = strings.TrimPrefix(u.Path, "/")
			options = u.RawQuery
		}
	case "mysql":
		// format: $Username:$Password@tcp(host:port)/db?options
		if i := strings.Index(dsn, "tcp("); i != -1 {
			start := i + len("tcp(")
			if j := strings.Index(dsn[start:], ")"); j != -1 {
				hp := dsn[start : start+j]
				if c := strings.LastIndex(hp, ":"); c != -1 {
					host = hp[:c]
					port = hp[c+1:]
				} else {
					host = hp
				}
			}
		}
		if parts := strings.SplitN(dsn, ")/", 2); len(parts) == 2 {
			rest := parts[1]
			if q := strings.Index(rest, "?"); q != -1 {
				db = rest[:q]
				options = rest[q+1:]
			} else {
				db = rest
			}
		}
	}
	return
}

func (s *Service) ensureGCPOauth2Client(pend *connector.PendingSecret, r *http.Request) error {
	if pend.OAuth2Config == nil {
		if pend.Connector.Secrets == nil {
			pend.OAuth2Config = client.NewGCloud()
		} else {
			pend.Connector.Secrets.SetTarget(pend.CredType)
			secrets, err := s.Secrets.Load(r.Context(), pend.Connector.Secrets)
			if err != nil {
				return err
			}
			cred, ok := secrets.Target.(*cred.Oauth2Config)
			if !ok {
				return fmt.Errorf("failed to load oauth2 config")
			}
			pend.OAuth2Config = &cred.Config
		}
	}
	return nil
}
