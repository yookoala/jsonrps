package jsonrps_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yookoala/jsonrps"
)

// compareIDs compares two ID values, handling JSON number conversion
func compareIDs(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Handle numeric conversions
	aFloat, aIsFloat := a.(float64)
	bFloat, bIsFloat := b.(float64)
	aInt, aIsInt := a.(int)
	bInt, bIsInt := b.(int)

	// Both are floats
	if aIsFloat && bIsFloat {
		return aFloat == bFloat
	}

	// Both are ints
	if aIsInt && bIsInt {
		return aInt == bInt
	}

	// One is float, one is int
	if aIsFloat && bIsInt {
		return aFloat == float64(bInt)
	}
	if aIsInt && bIsFloat {
		return float64(aInt) == bFloat
	}

	// For other types, use reflect.DeepEqual
	return reflect.DeepEqual(a, b)
}

func TestJSONRPCRequest_JSONEncoding(t *testing.T) {
	tests := []struct {
		name     string
		request  jsonrps.JSONRPCRequest
		expected string
	}{
		{
			name: "simple request with string ID",
			request: jsonrps.JSONRPCRequest{
				Version: "2.0",
				Method:  "subtract",
				Params:  json.RawMessage(`[42, 23]`),
				ID:      "1",
			},
			expected: `{"jsonrpc":"2.0","method":"subtract","params":[42, 23],"id":"1"}`,
		},
		{
			name: "request with numeric ID",
			request: jsonrps.JSONRPCRequest{
				Version: "2.0",
				Method:  "subtract",
				Params:  json.RawMessage(`{"subtrahend": 23, "minuend": 42}`),
				ID:      1,
			},
			expected: `{"jsonrpc":"2.0","method":"subtract","params":{"subtrahend": 23, "minuend": 42},"id":1}`,
		},
		{
			name: "notification without ID",
			request: jsonrps.JSONRPCRequest{
				Version: "2.0",
				Method:  "update",
				Params:  json.RawMessage(`[1,2,3,4,5]`),
			},
			expected: `{"jsonrpc":"2.0","method":"update","params":[1,2,3,4,5]}`,
		},
		{
			name: "request without params",
			request: jsonrps.JSONRPCRequest{
				Version: "2.0",
				Method:  "ping",
				ID:      "ping-1",
			},
			expected: `{"jsonrpc":"2.0","method":"ping","id":"ping-1"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test encoding
			data, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatalf("Failed to marshal JSONRPCRequest: %v", err)
			}

			// Compare JSON semantically by unmarshaling both
			var actual, expected interface{}
			if err := json.Unmarshal(data, &actual); err != nil {
				t.Fatalf("Failed to unmarshal actual JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expected); err != nil {
				t.Fatalf("Failed to unmarshal expected JSON: %v", err)
			}

			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("JSON mismatch.\nActual:   %s\nExpected: %s", string(data), tt.expected)
			}

			// Test round-trip decoding
			var decoded jsonrps.JSONRPCRequest
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal JSONRPCRequest: %v", err)
			}

			if decoded.Version != tt.request.Version {
				t.Errorf("Version mismatch: got %s, want %s", decoded.Version, tt.request.Version)
			}
			if decoded.Method != tt.request.Method {
				t.Errorf("Method mismatch: got %s, want %s", decoded.Method, tt.request.Method)
			}

			// Compare ID - handle JSON number conversion
			if !compareIDs(decoded.ID, tt.request.ID) {
				t.Errorf("ID mismatch: got %v (type %T), want %v (type %T)", decoded.ID, decoded.ID, tt.request.ID, tt.request.ID)
			}

			// Compare params by unmarshaling both
			if tt.request.Params != nil {
				var decodedParams, originalParams interface{}
				if err := json.Unmarshal(decoded.Params, &decodedParams); err != nil {
					t.Fatalf("Failed to unmarshal decoded params: %v", err)
				}
				if err := json.Unmarshal(tt.request.Params, &originalParams); err != nil {
					t.Fatalf("Failed to unmarshal original params: %v", err)
				}
				if !reflect.DeepEqual(decodedParams, originalParams) {
					t.Errorf("Params mismatch: got %v, want %v", decodedParams, originalParams)
				}
			}
		})
	}
}

func TestJSONRPCError_JSONEncoding(t *testing.T) {
	tests := []struct {
		name     string
		error    jsonrps.JSONRPCError
		expected string
	}{
		{
			name: "error with string data",
			error: jsonrps.JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
				Data:    "The method 'foobar' does not exist / is not available.",
			},
			expected: `{"code":-32601,"message":"Method not found","data":"The method 'foobar' does not exist / is not available."}`,
		},
		{
			name: "error with object data",
			error: jsonrps.JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    map[string]interface{}{"expected": "array", "got": "object"},
			},
			expected: `{"code":-32602,"message":"Invalid params","data":{"expected":"array","got":"object"}}`,
		},
		{
			name: "error without data",
			error: jsonrps.JSONRPCError{
				Code:    -32700,
				Message: "Parse error",
			},
			expected: `{"code":-32700,"message":"Parse error"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test encoding
			data, err := json.Marshal(tt.error)
			if err != nil {
				t.Fatalf("Failed to marshal JSONRPCError: %v", err)
			}

			// Compare JSON semantically
			var actual, expected interface{}
			if err := json.Unmarshal(data, &actual); err != nil {
				t.Fatalf("Failed to unmarshal actual JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expected); err != nil {
				t.Fatalf("Failed to unmarshal expected JSON: %v", err)
			}

			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("JSON mismatch.\nActual:   %s\nExpected: %s", string(data), tt.expected)
			}

			// Test round-trip decoding
			var decoded jsonrps.JSONRPCError
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal JSONRPCError: %v", err)
			}

			if decoded.Code != tt.error.Code {
				t.Errorf("Code mismatch: got %d, want %d", decoded.Code, tt.error.Code)
			}
			if decoded.Message != tt.error.Message {
				t.Errorf("Message mismatch: got %s, want %s", decoded.Message, tt.error.Message)
			}
			if !reflect.DeepEqual(decoded.Data, tt.error.Data) {
				t.Errorf("Data mismatch: got %v, want %v", decoded.Data, tt.error.Data)
			}
		})
	}
}

func TestJSONRPCResponse_JSONEncoding(t *testing.T) {
	tests := []struct {
		name     string
		response jsonrps.JSONRPCResponse
		expected string
	}{
		{
			name: "successful response",
			response: jsonrps.JSONRPCResponse{
				Version: "2.0",
				Result:  json.RawMessage(`19`),
				ID:      "1",
			},
			expected: `{"jsonrpc":"2.0","id":"1","result":19}`,
		},
		{
			name: "error response",
			response: jsonrps.JSONRPCResponse{
				Version: "2.0",
				Error: &jsonrps.JSONRPCError{
					Code:    -32601,
					Message: "Method not found",
				},
				ID: "1",
			},
			expected: `{"jsonrpc":"2.0","id":"1","error":{"code":-32601,"message":"Method not found"}}`,
		},
		{
			name: "subscription notification",
			response: jsonrps.JSONRPCResponse{
				Version: "2.0",
				Method:  "subscription",
				Params:  json.RawMessage(`{"subscription":"0x1","result":{"number":"0x1b4"}}`),
			},
			expected: `{"jsonrpc":"2.0","method":"subscription","params":{"subscription":"0x1","result":{"number":"0x1b4"}}}`,
		},
		{
			name: "response with complex result",
			response: jsonrps.JSONRPCResponse{
				Version: "2.0",
				Result:  json.RawMessage(`{"name":"John","age":30,"city":"New York"}`),
				ID:      123,
			},
			expected: `{"jsonrpc":"2.0","id":123,"result":{"name":"John","age":30,"city":"New York"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test encoding
			data, err := json.Marshal(tt.response)
			if err != nil {
				t.Fatalf("Failed to marshal JSONRPCResponse: %v", err)
			}

			// Compare JSON semantically
			var actual, expected interface{}
			if err := json.Unmarshal(data, &actual); err != nil {
				t.Fatalf("Failed to unmarshal actual JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expected); err != nil {
				t.Fatalf("Failed to unmarshal expected JSON: %v", err)
			}

			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("JSON mismatch.\nActual:   %s\nExpected: %s", string(data), tt.expected)
			}

			// Test round-trip decoding
			var decoded jsonrps.JSONRPCResponse
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal JSONRPCResponse: %v", err)
			}

			if decoded.Version != tt.response.Version {
				t.Errorf("Version mismatch: got %s, want %s", decoded.Version, tt.response.Version)
			}
			if !compareIDs(decoded.ID, tt.response.ID) {
				t.Errorf("ID mismatch: got %v (type %T), want %v (type %T)", decoded.ID, decoded.ID, tt.response.ID, tt.response.ID)
			}
			if decoded.Method != tt.response.Method {
				t.Errorf("Method mismatch: got %s, want %s", decoded.Method, tt.response.Method)
			}

			// Compare result by unmarshaling both if present
			if tt.response.Result != nil {
				var decodedResult, originalResult interface{}
				if err := json.Unmarshal(decoded.Result, &decodedResult); err != nil {
					t.Fatalf("Failed to unmarshal decoded result: %v", err)
				}
				if err := json.Unmarshal(tt.response.Result, &originalResult); err != nil {
					t.Fatalf("Failed to unmarshal original result: %v", err)
				}
				if !reflect.DeepEqual(decodedResult, originalResult) {
					t.Errorf("Result mismatch: got %v, want %v", decodedResult, originalResult)
				}
			}

			// Compare params by unmarshaling both if present
			if tt.response.Params != nil {
				var decodedParams, originalParams interface{}
				if err := json.Unmarshal(decoded.Params, &decodedParams); err != nil {
					t.Fatalf("Failed to unmarshal decoded params: %v", err)
				}
				if err := json.Unmarshal(tt.response.Params, &originalParams); err != nil {
					t.Fatalf("Failed to unmarshal original params: %v", err)
				}
				if !reflect.DeepEqual(decodedParams, originalParams) {
					t.Errorf("Params mismatch: got %v, want %v", decodedParams, originalParams)
				}
			}

			// Compare error
			if tt.response.Error != nil {
				if decoded.Error == nil {
					t.Error("Expected error but got nil")
				} else {
					if decoded.Error.Code != tt.response.Error.Code {
						t.Errorf("Error code mismatch: got %d, want %d", decoded.Error.Code, tt.response.Error.Code)
					}
					if decoded.Error.Message != tt.response.Error.Message {
						t.Errorf("Error message mismatch: got %s, want %s", decoded.Error.Message, tt.response.Error.Message)
					}
					if !reflect.DeepEqual(decoded.Error.Data, tt.response.Error.Data) {
						t.Errorf("Error data mismatch: got %v, want %v", decoded.Error.Data, tt.response.Error.Data)
					}
				}
			} else if decoded.Error != nil {
				t.Errorf("Expected no error but got %+v", decoded.Error)
			}
		})
	}
}

func TestJSONDecoding_EdgeCases(t *testing.T) {
	t.Run("null ID in request", func(t *testing.T) {
		jsonStr := `{"jsonrpc":"2.0","method":"test","id":null}`
		var req jsonrps.JSONRPCRequest
		err := json.Unmarshal([]byte(jsonStr), &req)
		if err != nil {
			t.Fatalf("Failed to unmarshal request with null ID: %v", err)
		}
		if req.ID != nil {
			t.Errorf("Expected nil ID, got %v", req.ID)
		}
	})

	t.Run("empty params", func(t *testing.T) {
		jsonStr := `{"jsonrpc":"2.0","method":"test","params":{},"id":"1"}`
		var req jsonrps.JSONRPCRequest
		err := json.Unmarshal([]byte(jsonStr), &req)
		if err != nil {
			t.Fatalf("Failed to unmarshal request with empty params: %v", err)
		}

		var params interface{}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			t.Fatalf("Failed to unmarshal params: %v", err)
		}

		expected := map[string]interface{}{}
		if !reflect.DeepEqual(params, expected) {
			t.Errorf("Expected empty object, got %v", params)
		}
	})

	t.Run("response with both result and error should decode", func(t *testing.T) {
		// This is technically invalid JSON-RPC but we should be able to decode it
		jsonStr := `{"jsonrpc":"2.0","id":"1","result":42,"error":{"code":-1,"message":"test"}}`
		var resp jsonrps.JSONRPCResponse
		err := json.Unmarshal([]byte(jsonStr), &resp)
		if err != nil {
			t.Fatalf("Failed to unmarshal response with both result and error: %v", err)
		}

		var result interface{}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("Failed to unmarshal result: %v", err)
		}
		if result != float64(42) { // JSON numbers become float64
			t.Errorf("Expected result 42, got %v", result)
		}

		if resp.Error.Code != -1 {
			t.Errorf("Expected error code -1, got %d", resp.Error.Code)
		}
	})
}

func TestJSONRPCResponse_MarshalResult(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "marshal string",
			input:    "hello world",
			expected: `"hello world"`,
		},
		{
			name:     "marshal number",
			input:    42,
			expected: `42`,
		},
		{
			name:     "marshal float",
			input:    3.14159,
			expected: `3.14159`,
		},
		{
			name:     "marshal boolean",
			input:    true,
			expected: `true`,
		},
		{
			name:     "marshal null",
			input:    nil,
			expected: `null`,
		},
		{
			name:     "marshal object",
			input:    map[string]interface{}{"name": "John", "age": 30},
			expected: `{"age":30,"name":"John"}`,
		},
		{
			name:     "marshal array",
			input:    []int{1, 2, 3},
			expected: `[1,2,3]`,
		},
		{
			name: "marshal complex object",
			input: struct {
				Name    string   `json:"name"`
				Age     int      `json:"age"`
				Hobbies []string `json:"hobbies"`
			}{
				Name:    "Alice",
				Age:     25,
				Hobbies: []string{"reading", "swimming"},
			},
			expected: `{"name":"Alice","age":25,"hobbies":["reading","swimming"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp jsonrps.JSONRPCResponse

			// Test MarshalResult
			err := resp.MarshalResult(tt.input)
			if err != nil {
				t.Fatalf("MarshalResult failed: %v", err)
			}

			// Compare the marshaled result
			var actual, expected interface{}
			if err := json.Unmarshal(resp.Result, &actual); err != nil {
				t.Fatalf("Failed to unmarshal actual result: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expected); err != nil {
				t.Fatalf("Failed to unmarshal expected result: %v", err)
			}

			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("Result mismatch.\nActual:   %s\nExpected: %s", string(resp.Result), tt.expected)
			}
		})
	}
}

func TestJSONRPCResponse_UnmarshalResult(t *testing.T) {
	tests := []struct {
		name     string
		result   string
		target   interface{}
		expected interface{}
	}{
		{
			name:     "unmarshal to string",
			result:   `"hello world"`,
			target:   new(string),
			expected: "hello world",
		},
		{
			name:     "unmarshal to int",
			result:   `42`,
			target:   new(int),
			expected: 42,
		},
		{
			name:     "unmarshal to float64",
			result:   `3.14159`,
			target:   new(float64),
			expected: 3.14159,
		},
		{
			name:     "unmarshal to bool",
			result:   `true`,
			target:   new(bool),
			expected: true,
		},
		{
			name:     "unmarshal to map",
			result:   `{"name":"John","age":30}`,
			target:   new(map[string]interface{}),
			expected: map[string]interface{}{"name": "John", "age": float64(30)}, // JSON numbers become float64
		},
		{
			name:     "unmarshal to slice",
			result:   `[1,2,3]`,
			target:   new([]any),
			expected: []any{float64(1), float64(2), float64(3)}, // JSON numbers become float64
		},
		{
			name:   "unmarshal to struct",
			result: `{"name":"Alice","age":25,"hobbies":["reading","swimming"]}`,
			target: new(struct {
				Name    string   `json:"name"`
				Age     int      `json:"age"`
				Hobbies []string `json:"hobbies"`
			}),
			expected: struct {
				Name    string   `json:"name"`
				Age     int      `json:"age"`
				Hobbies []string `json:"hobbies"`
			}{
				Name:    "Alice",
				Age:     25,
				Hobbies: []string{"reading", "swimming"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := jsonrps.JSONRPCResponse{
				Result: json.RawMessage(tt.result),
			}

			// Test UnmarshalResult
			err := resp.UnmarshalResult(tt.target)
			if err != nil {
				t.Fatalf("UnmarshalResult failed: %v", err)
			}

			// Get the actual value by dereferencing the pointer
			actual := reflect.ValueOf(tt.target).Elem().Interface()

			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("Unmarshal mismatch.\nActual:   %+v\nExpected: %+v", actual, tt.expected)
			}
		})
	}
}

func TestJSONRPCResponse_UnmarshalResult_NilResult(t *testing.T) {
	resp := jsonrps.JSONRPCResponse{
		Result: nil,
	}

	var target interface{}
	err := resp.UnmarshalResult(&target)
	if err != nil {
		t.Fatalf("UnmarshalResult with nil result failed: %v", err)
	}

	if target != nil {
		t.Errorf("Expected nil target, got %v", target)
	}
}

func TestJSONRPCResponse_MarshalParams(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "marshal string params",
			input:    "test notification",
			expected: `"test notification"`,
		},
		{
			name:     "marshal array params",
			input:    []any{"param1", 42, true},
			expected: `["param1",42,true]`,
		},
		{
			name:     "marshal object params",
			input:    map[string]interface{}{"method": "subscribe", "topic": "events"},
			expected: `{"method":"subscribe","topic":"events"}`,
		},
		{
			name:     "marshal null params",
			input:    nil,
			expected: `null`,
		},
		{
			name: "marshal subscription params",
			input: map[string]interface{}{
				"subscription": "sub-123",
				"result":       map[string]interface{}{"status": "active"},
			},
			expected: `{"result":{"status":"active"},"subscription":"sub-123"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp jsonrps.JSONRPCResponse

			// Test MarshalParams
			err := resp.MarshalParams(tt.input)
			if err != nil {
				t.Fatalf("MarshalParams failed: %v", err)
			}

			// Compare the marshaled params
			var actual, expected interface{}
			if err := json.Unmarshal(resp.Params, &actual); err != nil {
				t.Fatalf("Failed to unmarshal actual params: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expected); err != nil {
				t.Fatalf("Failed to unmarshal expected params: %v", err)
			}

			if !reflect.DeepEqual(actual, expected) {
				t.Errorf("Params mismatch.\nActual:   %s\nExpected: %s", string(resp.Params), tt.expected)
			}
		})
	}
}

func TestJSONRPCResponse_UnmarshalParams(t *testing.T) {
	tests := []struct {
		name     string
		params   string
		target   interface{}
		expected interface{}
	}{
		{
			name:     "unmarshal string params",
			params:   `"test notification"`,
			target:   new(string),
			expected: "test notification",
		},
		{
			name:     "unmarshal array params",
			params:   `["param1",42,true]`,
			target:   new([]any),
			expected: []any{"param1", float64(42), true}, // JSON numbers become float64
		},
		{
			name:     "unmarshal object params",
			params:   `{"method":"subscribe","topic":"events"}`,
			target:   new(map[string]interface{}),
			expected: map[string]interface{}{"method": "subscribe", "topic": "events"},
		},
		{
			name:   "unmarshal subscription params",
			params: `{"subscription":"sub-123","result":{"status":"active"}}`,
			target: new(struct {
				Subscription string                 `json:"subscription"`
				Result       map[string]interface{} `json:"result"`
			}),
			expected: struct {
				Subscription string                 `json:"subscription"`
				Result       map[string]interface{} `json:"result"`
			}{
				Subscription: "sub-123",
				Result:       map[string]interface{}{"status": "active"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := jsonrps.JSONRPCResponse{
				Params: json.RawMessage(tt.params),
			}

			// Test UnmarshalParams
			err := resp.UnmarshalParams(tt.target)
			if err != nil {
				t.Fatalf("UnmarshalParams failed: %v", err)
			}

			// Get the actual value by dereferencing the pointer
			actual := reflect.ValueOf(tt.target).Elem().Interface()

			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("Unmarshal mismatch.\nActual:   %+v\nExpected: %+v", actual, tt.expected)
			}
		})
	}
}

func TestJSONRPCResponse_UnmarshalParams_NilParams(t *testing.T) {
	resp := jsonrps.JSONRPCResponse{
		Params: nil,
	}

	var target interface{}
	err := resp.UnmarshalParams(&target)
	if err != nil {
		t.Fatalf("UnmarshalParams with nil params failed: %v", err)
	}

	if target != nil {
		t.Errorf("Expected nil target, got %v", target)
	}
}

func TestJSONRPCResponse_MarshalUnmarshal_RoundTrip(t *testing.T) {
	// Test round-trip marshal/unmarshal for both Result and Params
	t.Run("result round-trip", func(t *testing.T) {
		original := map[string]interface{}{
			"users": []any{
				map[string]interface{}{"id": 1, "name": "Alice"},
				map[string]interface{}{"id": 2, "name": "Bob"},
			},
			"total": 2,
		}

		var resp jsonrps.JSONRPCResponse

		// Marshal
		if err := resp.MarshalResult(original); err != nil {
			t.Fatalf("MarshalResult failed: %v", err)
		}

		// Unmarshal
		var decoded map[string]interface{}
		if err := resp.UnmarshalResult(&decoded); err != nil {
			t.Fatalf("UnmarshalResult failed: %v", err)
		}

		// Note: JSON numbers become float64, so we need to adjust expectations
		expected := map[string]interface{}{
			"users": []any{
				map[string]interface{}{"id": float64(1), "name": "Alice"},
				map[string]interface{}{"id": float64(2), "name": "Bob"},
			},
			"total": float64(2),
		}

		if !reflect.DeepEqual(decoded, expected) {
			t.Errorf("Round-trip result mismatch.\nActual:   %+v\nExpected: %+v", decoded, expected)
		}
	})

	t.Run("params round-trip", func(t *testing.T) {
		original := []any{"method_name", map[string]interface{}{"arg1": "value1", "arg2": 42}}

		var resp jsonrps.JSONRPCResponse

		// Marshal
		if err := resp.MarshalParams(original); err != nil {
			t.Fatalf("MarshalParams failed: %v", err)
		}

		// Unmarshal
		var decoded []any
		if err := resp.UnmarshalParams(&decoded); err != nil {
			t.Fatalf("UnmarshalParams failed: %v", err)
		}

		// Note: JSON numbers become float64
		expected := []any{"method_name", map[string]interface{}{"arg1": "value1", "arg2": float64(42)}}

		if !reflect.DeepEqual(decoded, expected) {
			t.Errorf("Round-trip params mismatch.\nActual:   %+v\nExpected: %+v", decoded, expected)
		}
	})
}

func TestJSONRPCResponse_MarshalErrors(t *testing.T) {
	// Test marshaling values that can't be marshaled to JSON
	t.Run("marshal result with channel", func(t *testing.T) {
		var resp jsonrps.JSONRPCResponse
		ch := make(chan int)
		defer close(ch)

		err := resp.MarshalResult(ch)
		if err == nil {
			t.Error("Expected error when marshaling channel, got nil")
		}
	})

	t.Run("marshal params with function", func(t *testing.T) {
		var resp jsonrps.JSONRPCResponse
		fn := func() {}

		err := resp.MarshalParams(fn)
		if err == nil {
			t.Error("Expected error when marshaling function, got nil")
		}
	})
}
