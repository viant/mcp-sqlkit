package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/viant/mcp-protocol/authorization"
	"github.com/viant/mcp-sqlkit/policy"
	"golang.org/x/oauth2"
)

// buildUnsignedJWT assembles an unsigned (algorithm "none") JWT with provided claims.
func buildUnsignedJWT(claims map[string]interface{}) string {
	header := []byte(`{"alg":"none","typ":"JWT"}`)
	headerEnc := base64.RawURLEncoding.EncodeToString(header)
	payloadData, _ := json.Marshal(claims)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadData)
	// Final dot is kept even though signature part is empty – this keeps the
	// token structurally valid for the parser.
	return fmt.Sprintf("%s.%s.", headerEnc, payloadEnc)
}

func TestService_Namespace(t *testing.T) {
	// Prepare sample OAuth2 config (values are irrelevant for this unit test).
	oauthCfg := &oauth2.Config{ClientID: "test-client"}

	// Build a dummy unsigned JWT.
	tokenString := buildUnsignedJWT(map[string]interface{}{
		"sub":   "sub123",
		"email": "user@example.com",
	})

	testCases := []struct {
		name        string
		policy      *policy.Policy
		ctx         context.Context
		expectNS    string
		expectError bool
	}{
		{
			name:     "no oauth config -> default namespace",
			policy:   &policy.Policy{},
			ctx:      context.Background(),
			expectNS: "default",
		},
		{
			name:        "oauth config but missing token -> error",
			policy:      &policy.Policy{Oauth2Config: oauthCfg},
			ctx:         context.Background(),
			expectError: true,
		},
		{
			name:   "oauth config with token -> derived namespace",
			policy: &policy.Policy{Oauth2Config: oauthCfg},
			ctx: context.WithValue(context.Background(), authorization.TokenKey, &authorization.Token{
				Token: tokenString,
			}),
			expectNS: "user@example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := New(tc.policy)
			ns, err := svc.Namespace(tc.ctx)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ns != tc.expectNS {
				t.Fatalf("namespace mismatch: expected %q, got %q", tc.expectNS, ns)
			}
		})
	}
}
