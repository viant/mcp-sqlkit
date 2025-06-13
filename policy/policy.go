package policy

import (
	"golang.org/x/oauth2"
)

type Policy struct {

	// Oauth2Config is used to generate tokens for the connection
	Oauth2Config *oauth2.Config `json:"oauth2,omitempty" yaml:"oauth2,omitempty"`

	// RequireIdentityToken indicates whether this policy mandates identity tokens
	RequireIdentityToken bool `json:"requireIdentityToken,omitempty" yaml:"requireIdentityToken,omitempty"`
}
