package connector

import "errors"

// Typed errors returned by Manager business logic.
// Service layer can analyse these error values to decide whether to trigger
// UI / elicitation workflows. Any external code should rely on the exported
// variables rather than string comparison.

var (
	// ErrNamespaceNotFound is returned when a namespace derived from the
	// request context has no connectors registered.
	ErrNamespaceNotFound = errors.New("namespace not found")

	// ErrConnectorNotFound is returned when the requested connector name is
	// missing in the namespace map.
	ErrConnectorNotFound = errors.New("connector not found")
)
