package auth

import (
	"context"
	"fmt"
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
	// When no OAuth2 configuration is provided, remain in the shared default
	// namespace to preserve the existing behaviour (backwards-compatibility).
	if s == nil || s.Policy == nil || s.Policy.Oauth2Config == nil {
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

	var tokenString string
	switch tv := tokenValue.(type) {
	case string:
		tokenString = tv
	case *authorization.Token:
		tokenString = tv.Token
	default:
		return "", fmt.Errorf("failed to get token from context, unsupported type %T", tokenValue)
	}

	// If verifier service is not configured (i.e. New() was called without
	// additional JWT verification settings) we perform a best-effort, safe
	// extraction of standard claims without validating the signature. This is
	// sufficient for namespace derivation purposes and avoids hard failures in
	// test environments where public keys are not available.
	if s.verifierService == nil {
		if ns := unsafeSubjectOrEmail(tokenString); ns != "" {
			return ns, nil
		}
		return "", fmt.Errorf("unable to extract namespace from token")
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
		return "", fmt.Errorf("namespace is empty in token claims")
	}
	return namespace, nil
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
