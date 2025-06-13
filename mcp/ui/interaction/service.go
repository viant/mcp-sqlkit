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
	"strings"
	"time"
)

//go:embed asset/basic_cred.html
var basicCred []byte

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

	s.renderBasicCred(w, map[string]string{
		"Dsn":          pend.Connector.DSN,
		"Connector":    pend.Connector.Name,
		"UUID":         pend.UUID,
		"ErrorMessage": "",
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
	s.handleCompletion(w, pend)
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
	username := data["username"]
	password := data["password"]

	if act, ok := data["action"]; ok && act == "cancel" {
		_ = s.Connector.CancelPending(r.Context(), pend.UUID)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><h3>Submission cancelled – you may close this tab.</h3></body></html>`))
		return
	}

	if username == "" {
		s.renderBasicCred(w, map[string]string{
			"Dsn":          pend.Connector.DSN,
			"Connector":    pend.Connector.Name,
			"UUID":         pend.UUID,
			"ErrorMessage": "Username is required",
		})
		return
	}
	if password == "" {
		s.renderBasicCred(w, map[string]string{
			"Dsn":          pend.Connector.DSN,
			"Connector":    pend.Connector.Name,
			"UUID":         pend.UUID,
			"ErrorMessage": "Password is required",
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
		})
		return
	}

	s.handleCompletion(w, pend)
}

func (s *Service) handleCompletion(w http.ResponseWriter, pend *connector.PendingSecret) {
	// Mark pending completed.
	_ = s.Connector.CompletePending(pend.UUID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<html><body><h3>Connector ready – you may close this tab.</h3></body></html>`))
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
