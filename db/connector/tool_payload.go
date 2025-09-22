package connector

// AddOutput conveys the result of attempting to register a connector via the
// dbSetConnection tool.
type AddOutput struct {
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	State       string `json:"state,omitempty"`
	CallbackURL string `json:"callbackURL,omitempty"`
	Connector   string `json:"connector,omitempty"`
}

// UpdateOutput is an alias kept for symmetry with AddOutput.
type UpdateOutput AddOutput
