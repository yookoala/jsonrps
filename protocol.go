package jsonrps

import "encoding/json"

// JSONRPCRequest represents a JSON-RPC 2.0 request object.
type JSONRPCRequest struct {
	// Version of the JSON-RPC protocol
	Version string `json:"jsonrpc"`

	// Method is the name of the method to be invoked
	Method string `json:"method"`

	// Params is the input parameters for the method
	Params json.RawMessage `json:"params,omitempty"`

	// ID is the unique identifier for the request
	ID any `json:"id,omitempty"`
}

// JSONRPCError represents the error object in a JSON-RPC 2.0 response.
type JSONRPCError struct {
	// Code is the error code
	Code int `json:"code"`

	// Message is the human-readable message of the error
	Message string `json:"message"`

	// Data is whatever additional data for the error
	Data any `json:"data,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response object.
type JSONRPCResponse struct {
	// Version of the JSON-RPC protocol
	Version string `json:"jsonrpc"`

	// ID is the unique identifier for the request
	ID any `json:"id,omitempty"`

	// Result is the successful response result
	Result json.RawMessage `json:"result,omitempty"`

	// Error is the error object in the response
	Error *JSONRPCError `json:"error,omitempty"`

	// -- Below are extended fields for subscription only --

	// Method is the name of the method for subscription notifications
	Method string `json:"method,omitempty"`

	// Params is the parameters for the subscription notification
	Params json.RawMessage `json:"params,omitempty"`
}

// MarshalResult convert v into JSON string and store in the Result field
func (r *JSONRPCResponse) MarshalResult(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	r.Result = data
	return nil
}

// UnmarshalResult converts the JSON string in the Result field back into the original value
func (r *JSONRPCResponse) UnmarshalResult(v any) error {
	if r.Result == nil {
		return json.Unmarshal([]byte("null"), v)
	}
	return json.Unmarshal(r.Result, v)
}

// MarshalParams converts v into JSON string and store in the Params field
func (r *JSONRPCResponse) MarshalParams(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	r.Params = data
	return nil
}

// UnmarshalParams converts the JSON string in the Params field back into the original value
func (r *JSONRPCResponse) UnmarshalParams(v any) error {
	if r.Params == nil {
		return json.Unmarshal([]byte("null"), v)
	}
	return json.Unmarshal(r.Params, v)
}
