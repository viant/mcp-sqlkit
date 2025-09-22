package mcp

import (
	"encoding/json"

	"github.com/viant/jsonrpc"
	"github.com/viant/mcp-protocol/schema"
)

// buildErrorResult constructs a CallToolResult with IsError set and the error
// message placed in the Text field (client-friendly).
func buildErrorResult(errMsg string) (*schema.CallToolResult, *jsonrpc.Error) {
	isErr := true
	return &schema.CallToolResult{
		IsError: &isErr,
		Content: []schema.CallToolResultContentElem{{Text: errMsg}},
	}, nil
}

// buildSuccessResult serialises `payload` to JSON and wraps it in a
// CallToolResult. If svc.UseTextField() is true the JSON is returned in the
// `text` field, otherwise it is placed in the `data` field.
func buildSuccessResult(svc *Service, payload any) (*schema.CallToolResult, *jsonrpc.Error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, jsonrpc.NewInternalError(err.Error(), nil)
	}

	if svc.UseTextField() {
		return &schema.CallToolResult{Content: []schema.CallToolResultContentElem{{Text: string(data)}}}, nil
	}
	return &schema.CallToolResult{Content: []schema.CallToolResultContentElem{{Data: string(data)}}}, nil
}
