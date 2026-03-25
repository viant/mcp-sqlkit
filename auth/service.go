package auth

import (
	"context"
	"crypto/md5"
	"fmt"
	"log"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/viant/mcp-protocol/authorization"
	"github.com/viant/mcp-sqlkit/policy"
	"github.com/viant/scy/auth/jwt/verifier"
)

var defaultNs = "default"

func IsDefaultNamespace(namespace string) bool {
	return namespace == defaultNs
}

type Service struct {
	Policy          *policy.Policy
	verifierService *verifier.Service
}

func (s *Service) Namespace(ctx context.Context) (string, error) {
	// If service is not initialised, fall back to shared default.
	if s == nil {
		logAccess("anonymous", defaultNs)
		return defaultNs, nil
	}
	// When no OAuth2 configuration is provided, remain in the shared default
	// namespace to preserve the existing behaviour (backwards-compatibility)
	// and to support stdio/local flows without tokens.
	if s.Policy == nil || s.Policy.Oauth2Config == nil {
		logAccess("anonymous", defaultNs)
		return defaultNs, nil
	}

	// Token is expected to be propagated from the HTTP layer by the MCP auth
	// middleware and stored in the context under authorization.TokenKey. The
	// value may be either a plain string (legacy) or *authorization.Token –
	// support both for forward/backward compatibility.
	tokenValue := ctx.Value(authorization.TokenKey)
	if tokenValue == nil {
		return "", fmt.Errorf("failed to get token from context: missing value")
	}

	tokenString, err := tokenStringFromContextValue(tokenValue)
	if err != nil {
		return "", err
	}

	// Strip optional "Bearer " prefix if present (case-insensitive).
	if ls := strings.ToLower(tokenString); strings.HasPrefix(ls, "bearer ") {
		tokenString = strings.TrimSpace(tokenString[len("Bearer "):])
	}

	user := principalFromToken(tokenString)
	if user == "" {
		user = "token"
	}

	// When OAuth is configured but the server is not requested to use ID tokens
	// (i.e. running with -o but without -i), we cannot derive a stable subject
	// from access tokens. In this case, scope the namespace by hashing the
	// token string to ensure per-token isolation.
	if s.Policy != nil && !s.Policy.RequireIdentityToken {
		sum := md5.Sum([]byte(tokenString))
		ns := fmt.Sprintf("%x", sum)
		logAccess(user, ns)
		return ns, nil
	}

	// If verifier service is not configured (i.e. New() was called without
	// additional JWT verification settings) we perform a best-effort, safe
	// extraction of standard claims without validating the signature. This is
	// sufficient for namespace derivation purposes and avoids hard failures in
	// test environments where public keys are not available.
	if s.verifierService == nil {
		if ns := unsafeSubjectOrEmail(tokenString); ns != "" {
			logAccess(user, ns)
			return ns, nil
		}
		// Fallback: no ID-token/claims available – isolate by hashing token string.
		sum := md5.Sum([]byte(tokenString))
		ns := fmt.Sprintf("%x", sum)
		logAccess(user, ns)
		return ns, nil
	}

	claims, err := s.verifierService.VerifyClaims(ctx, tokenString)
	if err != nil {
		return "", err
	}

	namespace := claims.Email
	if namespace == "" {
		namespace = claims.Subject
	}
	if namespace == "" {
		// Verified token but missing subject/email – use hash for isolation.
		sum := md5.Sum([]byte(tokenString))
		ns := fmt.Sprintf("%x", sum)
		logAccess(user, ns)
		return ns, nil
	}
	logAccess(user, namespace)
	return namespace, nil
}

func tokenStringFromContextValue(tokenValue interface{}) (string, error) {
	switch tv := tokenValue.(type) {
	case string:
		return tv, nil
	case *authorization.Token:
		return tv.Token, nil
	default:
		return "", fmt.Errorf("failed to get token from context, unsupported type %T", tokenValue)
	}
}

func principalFromToken(tokenString string) string {
	if tokenString == "" {
		return ""
	}
	var claimMap jwt.MapClaims
	_, _, err := new(jwt.Parser).ParseUnverified(tokenString, &claimMap)
	if err != nil {
		return ""
	}
	if email, _ := claimMap["email"].(string); email != "" {
		return email
	}
	if sub, _ := claimMap["sub"].(string); sub != "" {
		return sub
	}
	return ""
}

func logAccess(user, namespace string) {
	log.Printf("mcp-sqlkit access user=%q namespace=%q", user, namespace)
}

// unsafeSubjectOrEmail extracts the "sub" or "email" claim **without**
// verifying the token signature. This helper must only be used as a fallback
// when no verifier service is configured.
func unsafeSubjectOrEmail(tokenString string) string {
	// The JWT library used by scy offers an unverified parse helper – leverage
	// that to read standard claims. Any parsing error results in an empty
	// string, signalling failure to the caller while keeping the function side
	//-effect free.
	var claimMap jwt.MapClaims
	_, _, err := new(jwt.Parser).ParseUnverified(tokenString, &claimMap)
	if err != nil {
		return ""
	}
	if email, _ := claimMap["email"].(string); email != "" {
		return email
	}
	if sub, _ := claimMap["sub"].(string); sub != "" {
		return sub
	}
	return ""
}

func New(policy *policy.Policy) *Service {
	ret := &Service{Policy: policy}
	if policy.Oauth2Config != nil {
		//TODO load cert from authorization server if presents
	}
	return ret
}
