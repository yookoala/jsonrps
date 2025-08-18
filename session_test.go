package jsonrps_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/yookoala/jsonrps"
)

// Mock implementations for testing session interfaces

// mockReadWriteCloser is a mock implementation of io.ReadWriteCloser for testing
type mockReadWriteCloser struct {
	readData  string
	writeData strings.Builder
	closed    bool
}

func (m *mockReadWriteCloser) Read(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.EOF
	}
	n = copy(p, m.readData)
	m.readData = m.readData[n:]
	if len(m.readData) == 0 {
		return n, io.EOF
	}
	return n, nil
}

func (m *mockReadWriteCloser) Write(p []byte) (n int, err error) {
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return m.writeData.Write(p)
}

func (m *mockReadWriteCloser) Close() error {
	m.closed = true
	return nil
}

// mockSessionHandler is a mock implementation of SessionHandler for testing
type mockSessionHandler struct {
	handleSessionCalled bool
	lastSession         *jsonrps.Session
}

func (m *mockSessionHandler) HandleSession(session *jsonrps.Session) {
	m.handleSessionCalled = true
	m.lastSession = session
}

// mockServerSessionHandler is a mock implementation of ServerSessionHandler for testing
type mockServerSessionHandler struct {
	mockSessionHandler
	canHandle bool
}

func (m *mockServerSessionHandler) CanHandleSession(session *jsonrps.Session) bool {
	return m.canHandle
}

func TestSession_Creation(t *testing.T) {
	// Test creating a new Session with all fields
	ctx := context.Background()
	header := http.Header{
		"Content-Type": []string{"application/json"},
		"User-Agent":   []string{"test-client/1.0"},
	}
	conn := &mockReadWriteCloser{readData: "test data"}

	session := &jsonrps.Session{
		ProtocolSignature: "Test Protocol 1.0",
		Headers:           header,
		Context:           ctx,
		Conn:              conn,
	}

	// Verify all fields are set correctly
	if session.ProtocolSignature != "Test Protocol 1.0" {
		t.Errorf("Expected ProtocolSignature 'Test Protocol 1.0', got '%s'", session.ProtocolSignature)
	}

	if !reflect.DeepEqual(session.Headers, header) {
		t.Errorf("Expected Header %v, got %v", header, session.Headers)
	}

	if session.Context != ctx {
		t.Errorf("Expected Context %v, got %v", ctx, session.Context)
	}

	if session.Conn != conn {
		t.Errorf("Expected Conn %v, got %v", conn, session.Conn)
	}
}

func TestSession_WithEmptyFields(t *testing.T) {
	// Test creating a Session with nil/empty fields
	session := &jsonrps.Session{}

	if session.ProtocolSignature != "" {
		t.Errorf("Expected empty ProtocolSignature, got '%s'", session.ProtocolSignature)
	}

	if session.Headers != nil {
		t.Errorf("Expected nil Header, got %v", session.Headers)
	}

	if session.Context != nil {
		t.Errorf("Expected nil Context, got %v", session.Context)
	}

	if session.Conn != nil {
		t.Errorf("Expected nil Conn, got %v", session.Conn)
	}
}

func TestSession_FieldManipulation(t *testing.T) {
	session := &jsonrps.Session{}

	// Test setting and getting ProtocolSignature
	session.ProtocolSignature = "JSON-RPC 2.0"
	if session.ProtocolSignature != "JSON-RPC 2.0" {
		t.Errorf("Expected ProtocolSignature 'JSON-RPC 2.0', got '%s'", session.ProtocolSignature)
	}

	// Test setting and getting Header
	header := http.Header{"Authorization": []string{"Bearer token123"}}
	session.Headers = header
	if !reflect.DeepEqual(session.Headers, header) {
		t.Errorf("Expected Header %v, got %v", header, session.Headers)
	}

	// Test setting and getting Context
	ctx := context.WithValue(context.Background(), "key", "value")
	session.Context = ctx
	if session.Context != ctx {
		t.Errorf("Expected Context %v, got %v", ctx, session.Context)
	}

	// Test setting and getting Conn
	conn := &mockReadWriteCloser{}
	session.Conn = conn
	if session.Conn != conn {
		t.Errorf("Expected Conn %v, got %v", conn, session.Conn)
	}
}

func TestSession_HeaderOperations(t *testing.T) {
	session := &jsonrps.Session{
		Headers: make(http.Header),
	}

	// Test adding headers
	session.Headers.Add("Content-Type", "application/json")
	session.Headers.Add("Authorization", "Bearer abc123")

	if session.Headers.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", session.Headers.Get("Content-Type"))
	}

	if session.Headers.Get("Authorization") != "Bearer abc123" {
		t.Errorf("Expected Authorization 'Bearer abc123', got '%s'", session.Headers.Get("Authorization"))
	}

	// Test setting multiple values for same header
	session.Headers.Add("Accept", "application/json")
	session.Headers.Add("Accept", "text/plain")

	acceptValues := session.Headers["Accept"]
	expectedValues := []string{"application/json", "text/plain"}
	if !reflect.DeepEqual(acceptValues, expectedValues) {
		t.Errorf("Expected Accept values %v, got %v", expectedValues, acceptValues)
	}
}

func TestSessionHandler_Interface(t *testing.T) {
	// Test that our mock implements the SessionHandler interface
	var handler jsonrps.SessionHandler = &mockSessionHandler{}

	session := &jsonrps.Session{
		ProtocolSignature: "Test Protocol",
	}

	// Verify it can be called
	handler.HandleSession(session)

	// Verify the mock was called
	mockHandler := handler.(*mockSessionHandler)
	if !mockHandler.handleSessionCalled {
		t.Error("Expected HandleSession to be called")
	}

	if mockHandler.lastSession != session {
		t.Errorf("Expected lastSession to be %v, got %v", session, mockHandler.lastSession)
	}
}

func TestSessionHandler_MultipleHandlers(t *testing.T) {
	// Test multiple handlers handling the same session
	handler1 := &mockSessionHandler{}
	handler2 := &mockSessionHandler{}

	session := &jsonrps.Session{
		ProtocolSignature: "Multi Handler Test",
	}

	// Both handlers should be able to handle the session independently
	handler1.HandleSession(session)
	handler2.HandleSession(session)

	// Verify both handlers were called
	if !handler1.handleSessionCalled {
		t.Error("Expected first handler to be called")
	}
	if !handler2.handleSessionCalled {
		t.Error("Expected second handler to be called")
	}

	// Verify both handlers received the same session
	if handler1.lastSession != session {
		t.Errorf("Expected first handler's lastSession to be %v, got %v", session, handler1.lastSession)
	}
	if handler2.lastSession != session {
		t.Errorf("Expected second handler's lastSession to be %v, got %v", session, handler2.lastSession)
	}
}

func TestServerSessionHandler_Interface(t *testing.T) {
	// Test that our mock implements the ServerSessionHandler interface
	handler := &mockServerSessionHandler{canHandle: true}
	var serverHandler jsonrps.ServerSessionHandler = handler

	session := &jsonrps.Session{
		ProtocolSignature: "Test Protocol",
	}

	// Test CanHandleSession
	canHandle := serverHandler.CanHandleSession(session)
	if !canHandle {
		t.Error("Expected CanHandleSession to return true")
	}

	// Test HandleSession (inherited from SessionHandler)
	serverHandler.HandleSession(session)

	if !handler.handleSessionCalled {
		t.Error("Expected HandleSession to be called")
	}

	if handler.lastSession != session {
		t.Errorf("Expected lastSession to be %v, got %v", session, handler.lastSession)
	}
}

func TestServerSessionHandler_CannotHandle(t *testing.T) {
	// Test handler that cannot handle the session
	handler := &mockServerSessionHandler{canHandle: false}

	session := &jsonrps.Session{
		ProtocolSignature: "Unsupported Protocol",
	}

	// Test CanHandleSession returns false
	canHandle := handler.CanHandleSession(session)
	if canHandle {
		t.Error("Expected CanHandleSession to return false")
	}

	// Even if it can't handle, HandleSession should still work
	handler.HandleSession(session)

	if !handler.handleSessionCalled {
		t.Error("Expected HandleSession to be called even if handler can't handle")
	}
}

func TestServerSessionRouter_HandleSession_FirstMatch(t *testing.T) {
	// Create multiple handlers, only the first one can handle the session
	handler1 := &mockServerSessionHandler{canHandle: true}
	handler2 := &mockServerSessionHandler{canHandle: true}
	handler3 := &mockServerSessionHandler{canHandle: false}

	router := jsonrps.ServerSessionRouter{handler1, handler2, handler3}

	session := &jsonrps.Session{
		ProtocolSignature: "Test Protocol",
	}

	// Handle the session
	router.HandleSession(session)

	// Verify only the first handler was called
	if !handler1.handleSessionCalled {
		t.Error("Expected first handler to be called")
	}

	if handler2.handleSessionCalled {
		t.Error("Expected second handler NOT to be called")
	}

	if handler3.handleSessionCalled {
		t.Error("Expected third handler NOT to be called")
	}

	if handler1.lastSession != session {
		t.Errorf("Expected first handler's lastSession to be %v, got %v", session, handler1.lastSession)
	}
}

func TestServerSessionRouter_HandleSession_NoMatch(t *testing.T) {
	// Create handlers that cannot handle the session
	handler1 := &mockServerSessionHandler{canHandle: false}
	handler2 := &mockServerSessionHandler{canHandle: false}

	router := jsonrps.ServerSessionRouter{handler1, handler2}

	session := &jsonrps.Session{
		ProtocolSignature: "Unsupported Protocol",
	}

	// Handle the session
	router.HandleSession(session)

	// Verify no handlers were called
	if handler1.handleSessionCalled {
		t.Error("Expected first handler NOT to be called")
	}

	if handler2.handleSessionCalled {
		t.Error("Expected second handler NOT to be called")
	}
}

func TestServerSessionRouter_HandleSession_SecondMatch(t *testing.T) {
	// Create handlers where only the second one can handle the session
	handler1 := &mockServerSessionHandler{canHandle: false}
	handler2 := &mockServerSessionHandler{canHandle: true}
	handler3 := &mockServerSessionHandler{canHandle: true}

	router := jsonrps.ServerSessionRouter{handler1, handler2, handler3}

	session := &jsonrps.Session{
		ProtocolSignature: "Special Protocol",
	}

	// Handle the session
	router.HandleSession(session)

	// Verify only the second handler was called
	if handler1.handleSessionCalled {
		t.Error("Expected first handler NOT to be called")
	}

	if !handler2.handleSessionCalled {
		t.Error("Expected second handler to be called")
	}

	if handler3.handleSessionCalled {
		t.Error("Expected third handler NOT to be called")
	}

	if handler2.lastSession != session {
		t.Errorf("Expected second handler's lastSession to be %v, got %v", session, handler2.lastSession)
	}
}

func TestServerSessionRouter_EmptyRouter(t *testing.T) {
	// Test with empty router
	router := jsonrps.ServerSessionRouter{}

	session := &jsonrps.Session{
		ProtocolSignature: "Any Protocol",
	}

	// This should not panic and should do nothing
	router.HandleSession(session)
	// If we reach here without panic, the test passes
}

func TestServerSessionRouter_SingleHandler(t *testing.T) {
	// Test router with single handler
	handler := &mockServerSessionHandler{canHandle: true}
	router := jsonrps.ServerSessionRouter{handler}

	session := &jsonrps.Session{
		ProtocolSignature: "Single Handler Test",
	}

	router.HandleSession(session)

	if !handler.handleSessionCalled {
		t.Error("Expected single handler to be called")
	}

	if handler.lastSession != session {
		t.Errorf("Expected handler's lastSession to be %v, got %v", session, handler.lastSession)
	}
}

func TestSession_WithRealConnOperations(t *testing.T) {
	// Test Session with actual I/O operations on the connection
	conn := &mockReadWriteCloser{
		readData: "Hello, World!",
	}

	session := &jsonrps.Session{
		Conn: conn,
	}

	// Test writing to the connection
	data := []byte("test message")
	n, err := session.Conn.Write(data)
	if err != nil {
		t.Fatalf("Unexpected error writing to connection: %v", err)
	}
	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	// Verify the data was written
	if conn.writeData.String() != "test message" {
		t.Errorf("Expected written data 'test message', got '%s'", conn.writeData.String())
	}

	// Test reading from the connection
	readBuf := make([]byte, 13)
	n, err = session.Conn.Read(readBuf)
	if err != nil && err != io.EOF {
		t.Fatalf("Unexpected error reading from connection: %v", err)
	}
	if string(readBuf[:n]) != "Hello, World!" {
		t.Errorf("Expected to read 'Hello, World!', got '%s'", string(readBuf[:n]))
	}

	// Test closing the connection
	err = session.Conn.Close()
	if err != nil {
		t.Fatalf("Unexpected error closing connection: %v", err)
	}

	// Verify connection is closed
	_, err = session.Conn.Write([]byte("should fail"))
	if err == nil {
		t.Error("Expected error writing to closed connection")
	}
}

func TestSession_ContextOperations(t *testing.T) {
	// Test Session with context operations
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session := &jsonrps.Session{
		Context: ctx,
	}

	// Test that context is accessible
	select {
	case <-session.Context.Done():
		t.Error("Context should not be cancelled yet")
	default:
		// Expected - context is not cancelled
	}

	// Cancel the context
	cancel()

	// Test that context is now cancelled
	select {
	case <-session.Context.Done():
		// Expected - context is cancelled
	default:
		t.Error("Context should be cancelled")
	}

	// Test context value
	ctxWithValue := context.WithValue(context.Background(), "sessionID", "12345")
	session.Context = ctxWithValue

	if session.Context.Value("sessionID") != "12345" {
		t.Errorf("Expected context value '12345', got '%v'", session.Context.Value("sessionID"))
	}
}

func TestSession_ProtocolSignatureVariations(t *testing.T) {
	tests := []struct {
		name      string
		signature string
	}{
		{"empty signature", ""},
		{"simple signature", "JSON-RPC"},
		{"versioned signature", "JSON-RPC 2.0"},
		{"complex signature", "Doorbot Protocol v1.2.3"},
		{"signature with spaces", "  JSON RPC Protocol  "},
		{"unicode signature", "协议 1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &jsonrps.Session{
				ProtocolSignature: tt.signature,
			}

			if session.ProtocolSignature != tt.signature {
				t.Errorf("Expected ProtocolSignature '%s', got '%s'", tt.signature, session.ProtocolSignature)
			}
		})
	}
}

func TestMockReadWriteCloser_EdgeCases(t *testing.T) {
	// Test mock with empty read data
	conn := &mockReadWriteCloser{readData: ""}

	buf := make([]byte, 10)
	n, err := conn.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected EOF error, got %v", err)
	}
	if n != 0 {
		t.Errorf("Expected 0 bytes read, got %d", n)
	}

	// Test writing after close
	conn.Close()
	_, err = conn.Write([]byte("test"))
	if err == nil {
		t.Error("Expected error writing to closed connection")
	}

	// Test reading after close
	_, err = conn.Read(buf)
	if err != io.EOF {
		t.Errorf("Expected EOF when reading from closed connection, got %v", err)
	}
}

func TestServerSessionRouter_implementsServerSessionHandler(t *testing.T) {
	router := jsonrps.ServerSessionRouter{}
	if _, ok := interface{}(router).(jsonrps.ServerSessionHandler); !ok {
		t.Error("Expected ServerSessionRouter to implement ServerSessionHandler")
	}
}

func TestServerSessionRouter_HandlerOrder(t *testing.T) {
	// Test that handlers are checked in order
	var callOrder []int

	// Create custom handlers that track call order
	handler1 := &mockServerSessionHandlerWithOrder{id: 1, canHandle: false, callOrder: &callOrder}
	handler2 := &mockServerSessionHandlerWithOrder{id: 2, canHandle: false, callOrder: &callOrder}
	handler3 := &mockServerSessionHandlerWithOrder{id: 3, canHandle: true, callOrder: &callOrder}

	router := jsonrps.ServerSessionRouter{handler1, handler2, handler3}
	session := &jsonrps.Session{ProtocolSignature: "Test"}

	router.HandleSession(session)

	expectedOrder := []int{1, 2, 3}
	if !reflect.DeepEqual(callOrder, expectedOrder) {
		t.Errorf("Expected call order %v, got %v", expectedOrder, callOrder)
	}

	// Verify only handler3 actually handled the session
	if handler1.handleSessionCalled {
		t.Error("Expected handler1 NOT to handle session")
	}
	if handler2.handleSessionCalled {
		t.Error("Expected handler2 NOT to handle session")
	}
	if !handler3.handleSessionCalled {
		t.Error("Expected handler3 to handle session")
	}
}

// mockServerSessionHandlerWithOrder is a handler that tracks call order
type mockServerSessionHandlerWithOrder struct {
	mockSessionHandler
	id        int
	canHandle bool
	callOrder *[]int
}

func (m *mockServerSessionHandlerWithOrder) CanHandleSession(session *jsonrps.Session) bool {
	*m.callOrder = append(*m.callOrder, m.id)
	return m.canHandle
}

func TestServerSessionRouter_CanHandleSession_WithHandlers(t *testing.T) {
	// Test CanHandleSession when one of the handlers can handle the session
	handler1 := &mockServerSessionHandler{canHandle: false}
	handler2 := &mockServerSessionHandler{canHandle: true}
	handler3 := &mockServerSessionHandler{canHandle: false}

	router := jsonrps.ServerSessionRouter{handler1, handler2, handler3}
	session := &jsonrps.Session{ProtocolSignature: "Test Protocol"}

	result := router.CanHandleSession(session)

	if !result {
		t.Error("Expected router.CanHandleSession to return true when at least one handler can handle")
	}
}

func TestServerSessionRouter_CanHandleSession_NoHandlers(t *testing.T) {
	// Test CanHandleSession when no handlers can handle the session
	handler1 := &mockServerSessionHandler{canHandle: false}
	handler2 := &mockServerSessionHandler{canHandle: false}
	handler3 := &mockServerSessionHandler{canHandle: false}

	router := jsonrps.ServerSessionRouter{handler1, handler2, handler3}
	session := &jsonrps.Session{ProtocolSignature: "Test Protocol"}

	result := router.CanHandleSession(session)

	if result {
		t.Error("Expected router.CanHandleSession to return false when no handlers can handle")
	}
}

func TestServerSessionRouter_CanHandleSession_EmptyRouter(t *testing.T) {
	// Test CanHandleSession with empty router
	router := jsonrps.ServerSessionRouter{}
	session := &jsonrps.Session{ProtocolSignature: "Test Protocol"}

	result := router.CanHandleSession(session)

	if result {
		t.Error("Expected empty router.CanHandleSession to return false")
	}
}

func TestServerSessionRouter_CanHandleSession_FirstHandlerMatches(t *testing.T) {
	// Test CanHandleSession when the first handler can handle (should return true immediately)
	handler1 := &mockServerSessionHandler{canHandle: true}
	handler2 := &mockServerSessionHandler{canHandle: false}
	handler3 := &mockServerSessionHandler{canHandle: false}

	router := jsonrps.ServerSessionRouter{handler1, handler2, handler3}
	session := &jsonrps.Session{ProtocolSignature: "Test Protocol"}

	result := router.CanHandleSession(session)

	if !result {
		t.Error("Expected router.CanHandleSession to return true when first handler can handle")
	}
}

func TestServerSessionRouter_CanHandleSession_LastHandlerMatches(t *testing.T) {
	// Test CanHandleSession when only the last handler can handle
	handler1 := &mockServerSessionHandler{canHandle: false}
	handler2 := &mockServerSessionHandler{canHandle: false}
	handler3 := &mockServerSessionHandler{canHandle: true}

	router := jsonrps.ServerSessionRouter{handler1, handler2, handler3}
	session := &jsonrps.Session{ProtocolSignature: "Test Protocol"}

	result := router.CanHandleSession(session)

	if !result {
		t.Error("Expected router.CanHandleSession to return true when last handler can handle")
	}
}

func TestServerSessionRouter_CanHandleSession_MultipleMatches(t *testing.T) {
	// Test CanHandleSession when multiple handlers can handle (should still return true)
	handler1 := &mockServerSessionHandler{canHandle: true}
	handler2 := &mockServerSessionHandler{canHandle: true}
	handler3 := &mockServerSessionHandler{canHandle: false}

	router := jsonrps.ServerSessionRouter{handler1, handler2, handler3}
	session := &jsonrps.Session{ProtocolSignature: "Test Protocol"}

	result := router.CanHandleSession(session)

	if !result {
		t.Error("Expected router.CanHandleSession to return true when multiple handlers can handle")
	}
}

func TestServerSessionRouter_CanHandleSession_ChecksAllHandlers(t *testing.T) {
	// Test that CanHandleSession checks all handlers until it finds one that can handle
	var checkOrder []int

	handler1 := &mockServerSessionHandlerWithOrder{id: 1, canHandle: false, callOrder: &checkOrder}
	handler2 := &mockServerSessionHandlerWithOrder{id: 2, canHandle: false, callOrder: &checkOrder}
	handler3 := &mockServerSessionHandlerWithOrder{id: 3, canHandle: true, callOrder: &checkOrder}
	handler4 := &mockServerSessionHandlerWithOrder{id: 4, canHandle: true, callOrder: &checkOrder}

	router := jsonrps.ServerSessionRouter{handler1, handler2, handler3, handler4}
	session := &jsonrps.Session{ProtocolSignature: "Test Protocol"}

	result := router.CanHandleSession(session)

	if !result {
		t.Error("Expected router.CanHandleSession to return true")
	}

	// Should have checked handlers 1, 2, and 3 (but not 4, since 3 already matched)
	expectedOrder := []int{1, 2, 3}
	if !reflect.DeepEqual(checkOrder, expectedOrder) {
		t.Errorf("Expected check order %v, got %v", expectedOrder, checkOrder)
	}
}

func TestSession_WriteHeader(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		headers    http.Header
		expected   string
	}{
		{
			name:       "200 OK with no headers",
			statusCode: 200,
			headers:    http.Header{},
			expected:   "JSONRPS/1.0 200 OK\r\n\r\n",
		},
		{
			name:       "404 Not Found with headers",
			statusCode: 404,
			headers: http.Header{
				"Content-Type": []string{"application/json"},
				"Server":       []string{"test-server/1.0"},
			},
			expected: "JSONRPS/1.0 404 Not Found\r\nContent-Type: application/json\r\nServer: test-server/1.0\r\n\r\n",
		},
		{
			name:       "500 Internal Server Error",
			statusCode: 500,
			headers:    http.Header{},
			expected:   "JSONRPS/1.0 500 Internal Server Error\r\n\r\n",
		},
		{
			name:       "Custom status with multiple header values",
			statusCode: 201,
			headers: http.Header{
				"Set-Cookie": []string{"session=abc123", "theme=dark"},
				"Location":   []string{"/new-resource"},
			},
			expected: "JSONRPS/1.0 201 Created\r\nLocation: /new-resource\r\nSet-Cookie: session=abc123\r\nSet-Cookie: theme=dark\r\n\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &mockReadWriteCloser{}
			session := &jsonrps.Session{
				Headers: tt.headers,
				Conn:    conn,
			}

			session.WriteHeader(tt.statusCode)

			written := conn.writeData.String()
			// Since header order might vary, we need to verify components
			lines := strings.Split(written, "\r\n")

			// Check status line
			expectedStatusLine := fmt.Sprintf("JSONRPS/1.0 %d %s", tt.statusCode, http.StatusText(tt.statusCode))
			if lines[0] != expectedStatusLine {
				t.Errorf("Expected status line '%s', got '%s'", expectedStatusLine, lines[0])
			}

			// Check that it ends with double CRLF
			if !strings.HasSuffix(written, "\r\n\r\n") {
				t.Error("Expected response to end with double CRLF")
			}

			// Check headers are present
			for key, values := range tt.headers {
				for _, value := range values {
					expectedHeader := fmt.Sprintf("%s: %s", key, value)
					if !strings.Contains(written, expectedHeader) {
						t.Errorf("Expected header '%s' to be present in output", expectedHeader)
					}
				}
			}
		})
	}
}

func TestSession_Write(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected string
	}{
		{
			name:     "write simple string",
			data:     []byte("Hello, World!"),
			expected: "\r\nHello, World!",
		},
		{
			name:     "write JSON data",
			data:     []byte(`{"message": "test", "id": 123}`),
			expected: "\r\n" + `{"message": "test", "id": 123}`,
		},
		{
			name:     "write empty data",
			data:     []byte(""),
			expected: "\r\n",
		},
		{
			name:     "write binary data",
			data:     []byte{0x01, 0x02, 0x03, 0xFF},
			expected: "\r\n" + string([]byte{0x01, 0x02, 0x03, 0xFF}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &mockReadWriteCloser{}
			session := &jsonrps.Session{
				Conn: conn,
			}

			n, err := session.Write(tt.data)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if n != len(tt.data) {
				t.Errorf("Expected to write %d bytes, wrote %d", len(tt.data), n)
			}

			written := conn.writeData.String()
			if written != tt.expected {
				t.Errorf("Expected written data '%s', got '%s'", tt.expected, written)
			}
		})
	}
}

func TestSession_Write_WithHeadersAlreadySent(t *testing.T) {
	// Test writing when headers have already been sent via WriteHeader
	conn := &mockReadWriteCloser{}
	session := &jsonrps.Session{
		Headers: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Conn: conn,
	}

	// Send headers first
	session.WriteHeader(200)

	// Now write data - should not prepend \r\n since headers already sent
	data := []byte("Hello, World!")
	n, err := session.Write(data)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if n != len(data) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(data), n)
	}

	written := conn.writeData.String()
	// Should contain the full response: status line + headers + double CRLF + data
	expectedDataPart := "Hello, World!"
	if !strings.HasSuffix(written, expectedDataPart) {
		t.Errorf("Expected data '%s' to be at end of response, got '%s'", expectedDataPart, written)
	}

	// Should not have extra \r\n before the data since headers were already sent
	lines := strings.Split(written, "\r\n\r\n")
	if len(lines) != 2 {
		t.Errorf("Expected response to have exactly one double CRLF separator, got %d parts", len(lines))
	}

	if lines[1] != expectedDataPart {
		t.Errorf("Expected data part to be '%s', got '%s'", expectedDataPart, lines[1])
	}
}

func TestSession_Write_MultipleWrites(t *testing.T) {
	// Test multiple writes - only first should prepend \r\n
	conn := &mockReadWriteCloser{}
	session := &jsonrps.Session{
		Conn: conn,
	}

	// First write should prepend \r\n
	data1 := []byte("Hello")
	n1, err := session.Write(data1)
	if err != nil {
		t.Fatalf("Unexpected error on first write: %v", err)
	}
	if n1 != len(data1) {
		t.Errorf("Expected to write %d bytes on first write, wrote %d", len(data1), n1)
	}

	// Second write should NOT prepend \r\n
	data2 := []byte(", World!")
	n2, err := session.Write(data2)
	if err != nil {
		t.Fatalf("Unexpected error on second write: %v", err)
	}
	if n2 != len(data2) {
		t.Errorf("Expected to write %d bytes on second write, wrote %d", len(data2), n2)
	}

	written := conn.writeData.String()
	expected := "\r\nHello, World!"
	if written != expected {
		t.Errorf("Expected written data '%s', got '%s'", expected, written)
	}
}

func TestSession_Write_Error(t *testing.T) {
	// Test writing to closed connection
	conn := &mockReadWriteCloser{}
	conn.Close()

	session := &jsonrps.Session{
		Conn: conn,
	}

	_, err := session.Write([]byte("test"))
	if err == nil {
		t.Error("Expected error when writing to closed connection")
	}
}

func TestSession_Header(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		expected http.Header
	}{
		{
			name:     "empty headers",
			headers:  http.Header{},
			expected: http.Header{},
		},
		{
			name: "single header",
			headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			expected: http.Header{
				"Content-Type": []string{"application/json"},
			},
		},
		{
			name: "multiple headers",
			headers: http.Header{
				"Content-Type":  []string{"application/json"},
				"Authorization": []string{"Bearer token123"},
				"Accept":        []string{"application/json", "text/plain"},
			},
			expected: http.Header{
				"Content-Type":  []string{"application/json"},
				"Authorization": []string{"Bearer token123"},
				"Accept":        []string{"application/json", "text/plain"},
			},
		},
		{
			name:     "nil headers",
			headers:  nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &jsonrps.Session{
				Headers: tt.headers,
			}

			result := session.Header()

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected headers %v, got %v", tt.expected, result)
			}

			// Verify it returns the same reference (not a copy) for non-nil headers
			if tt.headers != nil {
				// Test that modifications to the returned header affect the session
				originalLen := len(session.Headers)
				result.Set("Test-Header", "test-value")
				if len(session.Headers) == originalLen && session.Headers.Get("Test-Header") != "test-value" {
					t.Error("Expected Header() to return reference to same header map")
				}
			}
		})
	}
}

func TestSession_Header_Modification(t *testing.T) {
	// Test that modifying the returned header affects the session
	session := &jsonrps.Session{
		Headers: make(http.Header),
	}

	header := session.Header()
	header.Set("Content-Type", "application/json")
	header.Add("Accept", "text/plain")

	// Verify the session's headers were modified
	if session.Headers.Get("Content-Type") != "application/json" {
		t.Error("Expected session headers to be modified when Header() result is modified")
	}

	if session.Headers.Get("Accept") != "text/plain" {
		t.Error("Expected session headers to be modified when Header() result is modified")
	}
}

func TestSession_WriteHeader_And_Write_Integration(t *testing.T) {
	// Test using WriteHeader followed by Write
	conn := &mockReadWriteCloser{}
	session := &jsonrps.Session{
		Headers: http.Header{
			"Content-Type":   []string{"application/json"},
			"Content-Length": []string{"26"},
		},
		Conn: conn,
	}

	// Write header first
	session.WriteHeader(200)

	// Then write body
	body := []byte(`{"status": "success"}`)
	n, err := session.Write(body)
	if err != nil {
		t.Fatalf("Unexpected error writing body: %v", err)
	}
	if n != len(body) {
		t.Errorf("Expected to write %d bytes, wrote %d", len(body), n)
	}

	// Verify complete response
	written := conn.writeData.String()

	// Should contain status line
	if !strings.Contains(written, "JSONRPS/1.0 200 OK") {
		t.Error("Expected status line in response")
	}

	// Should contain headers
	if !strings.Contains(written, "Content-Type: application/json") {
		t.Error("Expected Content-Type header in response")
	}

	// Should contain body
	if !strings.Contains(written, `{"status": "success"}`) {
		t.Error("Expected JSON body in response")
	}

	// Should have proper structure (headers before body)
	parts := strings.Split(written, "\r\n\r\n")
	if len(parts) != 2 {
		t.Errorf("Expected response to have header and body sections separated by double CRLF, got %d parts", len(parts))
	}

	if !strings.Contains(parts[1], `{"status": "success"}`) {
		t.Error("Expected body to be in second part after double CRLF")
	}
}

func TestSession_HeaderSent_Flag(t *testing.T) {
	// Test that headerSent flag works correctly
	t.Run("initial state", func(t *testing.T) {
		session := &jsonrps.Session{}
		// Note: headerSent is private, so we test behavior indirectly

		// Write should prepend \r\n on first call
		conn := &mockReadWriteCloser{}
		session.Conn = conn

		session.Write([]byte("test"))
		written := conn.writeData.String()
		if !strings.HasPrefix(written, "\r\n") {
			t.Error("Expected first Write to prepend \\r\\n when headers not sent")
		}
	})

	t.Run("after WriteHeader", func(t *testing.T) {
		conn := &mockReadWriteCloser{}
		session := &jsonrps.Session{
			Headers: http.Header{"Content-Type": []string{"text/plain"}},
			Conn:    conn,
		}

		// Call WriteHeader first
		session.WriteHeader(200)
		initialWrite := conn.writeData.String()

		// Now Write should not prepend \r\n
		session.Write([]byte("test"))
		finalWrite := conn.writeData.String()

		// The difference should be exactly "test" (no extra \r\n)
		addedContent := finalWrite[len(initialWrite):]
		if addedContent != "test" {
			t.Errorf("Expected Write after WriteHeader to add only 'test', got '%s'", addedContent)
		}
	})

	t.Run("after first Write", func(t *testing.T) {
		conn := &mockReadWriteCloser{}
		session := &jsonrps.Session{Conn: conn}

		// First Write should prepend \r\n
		session.Write([]byte("first"))
		firstWrite := conn.writeData.String()
		if firstWrite != "\r\nfirst" {
			t.Errorf("Expected first write to be '\\r\\nfirst', got '%s'", firstWrite)
		}

		// Second Write should not prepend \r\n
		session.Write([]byte("second"))
		finalWrite := conn.writeData.String()
		if finalWrite != "\r\nfirstsecond" {
			t.Errorf("Expected final write to be '\\r\\nfirstsecond', got '%s'", finalWrite)
		}
	})
}

func TestSession_WriteHeader_Multiple_Calls(t *testing.T) {
	// Test multiple calls to WriteHeader (should send headers each time)
	conn := &mockReadWriteCloser{}
	session := &jsonrps.Session{
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Conn:    conn,
	}

	// First WriteHeader call
	session.WriteHeader(200)
	firstResponse := conn.writeData.String()

	// Second WriteHeader call
	session.WriteHeader(404)
	secondResponse := conn.writeData.String()

	// Both responses should be present
	if !strings.Contains(firstResponse, "200 OK") {
		t.Error("Expected first response to contain '200 OK'")
	}

	if !strings.Contains(secondResponse, "404 Not Found") {
		t.Error("Expected second response to contain '404 Not Found'")
	}

	// Should have two complete header sections
	headerSections := strings.Split(secondResponse, "\r\n\r\n")
	if len(headerSections) < 3 { // first headers + second headers + empty body section
		t.Errorf("Expected at least 3 sections after double WriteHeader calls, got %d", len(headerSections))
	}
}

func TestSession_Header_Modification_After_WriteHeader(t *testing.T) {
	// Test that header modifications after WriteHeader don't affect sent headers
	conn := &mockReadWriteCloser{}
	session := &jsonrps.Session{
		Headers: make(http.Header),
		Conn:    conn,
	}

	// Set initial header
	session.Headers.Set("Content-Type", "application/json")

	// Send headers
	session.WriteHeader(200)
	afterHeaderWrite := conn.writeData.String()

	// Modify headers after sending
	session.Headers.Set("Content-Type", "text/plain")
	session.Headers.Set("New-Header", "new-value")

	// Write body
	session.Write([]byte("test body"))
	finalResponse := conn.writeData.String()

	// Original headers should be in response
	if !strings.Contains(afterHeaderWrite, "Content-Type: application/json") {
		t.Error("Expected original Content-Type header in sent response")
	}

	// New headers should NOT appear in the already-sent response
	if strings.Contains(finalResponse, "Content-Type: text/plain") {
		t.Error("Modified headers should not appear in already-sent response")
	}

	if strings.Contains(finalResponse, "New-Header: new-value") {
		t.Error("New headers should not appear in already-sent response")
	}
}

func TestSession_WriteRequest(t *testing.T) {
	tests := []struct {
		name     string
		request  *jsonrps.JSONRPCRequest
		expected string
	}{
		{
			name: "simple request",
			request: &jsonrps.JSONRPCRequest{
				Version: "2.0",
				Method:  "test.method",
				ID:      "123",
			},
			expected: `{"jsonrpc":"2.0","method":"test.method","id":"123"}` + "\n",
		},
		{
			name: "request with params",
			request: &jsonrps.JSONRPCRequest{
				Version: "2.0",
				Method:  "math.add",
				Params:  json.RawMessage(`[1, 2]`),
				ID:      42,
			},
			expected: `{"jsonrpc":"2.0","method":"math.add","params":[1,2],"id":42}` + "\n",
		},
		{
			name: "notification request (no ID)",
			request: &jsonrps.JSONRPCRequest{
				Version: "2.0",
				Method:  "notify.event",
				Params:  json.RawMessage(`{"event": "test"}`),
			},
			expected: `{"jsonrpc":"2.0","method":"notify.event","params":{"event":"test"}}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &mockReadWriteCloser{}
			session := &jsonrps.Session{
				Conn: conn,
			}

			err := session.WriteRequest(tt.request)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			written := conn.writeData.String()
			// Remove the automatic \r\n prefix that Write() adds for headers
			written = strings.TrimPrefix(written, "\r\n")

			if written != tt.expected {
				t.Errorf("Expected written data %q, got %q", tt.expected, written)
			}
		})
	}
}

func TestSession_WriteRequest_Error(t *testing.T) {
	// Test error handling when marshaling fails
	// Create a request with an unmarshalable field (function)
	conn := &mockReadWriteCloser{}
	session := &jsonrps.Session{
		Conn: conn,
	}

	// Test with closed connection
	conn.Close()
	request := &jsonrps.JSONRPCRequest{
		Version: "2.0",
		Method:  "test.method",
		ID:      "123",
	}

	err := session.WriteRequest(request)
	if err == nil {
		t.Error("Expected error when writing to closed connection")
	}
}

func TestSession_ReadRequest(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *jsonrps.JSONRPCRequest
		wantErr  bool
	}{
		{
			name:  "simple request",
			input: `{"jsonrpc":"2.0","method":"test.method","id":"123"}` + "\n",
			expected: &jsonrps.JSONRPCRequest{
				Version: "2.0",
				Method:  "test.method",
				ID:      "123",
			},
			wantErr: false,
		},
		{
			name:  "request with params",
			input: `{"jsonrpc":"2.0","method":"math.add","params":[1,2],"id":42}` + "\n",
			expected: &jsonrps.JSONRPCRequest{
				Version: "2.0",
				Method:  "math.add",
				Params:  json.RawMessage(`[1,2]`),
				ID:      float64(42), // JSON unmarshaling gives float64 for numbers
			},
			wantErr: false,
		},
		{
			name:  "notification request",
			input: `{"jsonrpc":"2.0","method":"notify.event","params":{"event":"test"}}` + "\n",
			expected: &jsonrps.JSONRPCRequest{
				Version: "2.0",
				Method:  "notify.event",
				Params:  json.RawMessage(`{"event":"test"}`),
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `{"invalid": json}` + "\n",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &mockReadWriteCloser{
				readData: tt.input,
			}
			session := &jsonrps.Session{
				Conn: conn,
			}

			request, err := session.ReadRequest()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if request == nil {
				t.Fatal("Expected request but got nil")
			}

			if request.Version != tt.expected.Version {
				t.Errorf("Expected Version %q, got %q", tt.expected.Version, request.Version)
			}

			if request.Method != tt.expected.Method {
				t.Errorf("Expected Method %q, got %q", tt.expected.Method, request.Method)
			}

			if !reflect.DeepEqual(request.ID, tt.expected.ID) {
				t.Errorf("Expected ID %v, got %v", tt.expected.ID, request.ID)
			}

			if len(tt.expected.Params) > 0 {
				if !bytes.Equal(request.Params, tt.expected.Params) {
					t.Errorf("Expected Params %s, got %s", tt.expected.Params, request.Params)
				}
			}
		})
	}
}

func TestSession_ReadRequest_Error(t *testing.T) {
	// Test read error
	conn := &mockReadWriteCloser{
		readData: "", // Empty data causes EOF immediately
	}
	session := &jsonrps.Session{
		Conn: conn,
	}

	request, err := session.ReadRequest()
	if err == nil {
		t.Error("Expected error when reading from connection with no data")
	}

	if request != nil {
		t.Error("Expected nil request when error occurs")
	}
}

func TestSession_WriteResponse(t *testing.T) {
	tests := []struct {
		name     string
		response *jsonrps.JSONRPCResponse
		expected string
	}{
		{
			name: "success response",
			response: &jsonrps.JSONRPCResponse{
				Version: "2.0",
				ID:      "123",
				Result:  json.RawMessage(`"success"`),
			},
			expected: `{"jsonrpc":"2.0","id":"123","result":"success"}` + "\n",
		},
		{
			name: "error response",
			response: &jsonrps.JSONRPCResponse{
				Version: "2.0",
				ID:      42,
				Error: &jsonrps.JSONRPCError{
					Code:    -32600,
					Message: "Invalid Request",
				},
			},
			expected: `{"jsonrpc":"2.0","id":42,"error":{"code":-32600,"message":"Invalid Request"}}` + "\n",
		},
		{
			name: "response with complex result",
			response: &jsonrps.JSONRPCResponse{
				Version: "2.0",
				ID:      "test",
				Result:  json.RawMessage(`{"status":"ok","data":[1,2,3]}`),
			},
			expected: `{"jsonrpc":"2.0","id":"test","result":{"status":"ok","data":[1,2,3]}}` + "\n",
		},
		{
			name: "notification response (no ID)",
			response: &jsonrps.JSONRPCResponse{
				Version: "2.0",
				Result:  json.RawMessage(`null`),
			},
			expected: `{"jsonrpc":"2.0","result":null}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &mockReadWriteCloser{}
			session := &jsonrps.Session{
				Conn: conn,
			}

			err := session.WriteResponse(tt.response)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			written := conn.writeData.String()
			// Remove the automatic \r\n prefix that Write() adds for headers
			written = strings.TrimPrefix(written, "\r\n")

			if written != tt.expected {
				t.Errorf("Expected written data %q, got %q", tt.expected, written)
			}
		})
	}
}

func TestSession_WriteResponse_Error(t *testing.T) {
	// Test error handling when writing to closed connection
	conn := &mockReadWriteCloser{}
	conn.Close()

	session := &jsonrps.Session{
		Conn: conn,
	}

	response := &jsonrps.JSONRPCResponse{
		Version: "2.0",
		ID:      "123",
		Result:  json.RawMessage(`"test"`),
	}

	err := session.WriteResponse(response)
	if err == nil {
		t.Error("Expected error when writing to closed connection")
	}
}

func TestSession_ReadResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *jsonrps.JSONRPCResponse
		wantErr  bool
	}{
		{
			name:  "success response",
			input: `{"jsonrpc":"2.0","id":"123","result":"success"}` + "\n",
			expected: &jsonrps.JSONRPCResponse{
				Version: "2.0",
				ID:      "123",
				Result:  json.RawMessage(`"success"`),
			},
			wantErr: false,
		},
		{
			name:  "error response",
			input: `{"jsonrpc":"2.0","id":42,"error":{"code":-32600,"message":"Invalid Request"}}` + "\n",
			expected: &jsonrps.JSONRPCResponse{
				Version: "2.0",
				ID:      float64(42), // JSON unmarshaling gives float64 for numbers
				Error: &jsonrps.JSONRPCError{
					Code:    -32600,
					Message: "Invalid Request",
				},
			},
			wantErr: false,
		},
		{
			name:  "response with complex result",
			input: `{"jsonrpc":"2.0","id":"test","result":{"status":"ok","data":[1,2,3]}}` + "\n",
			expected: &jsonrps.JSONRPCResponse{
				Version: "2.0",
				ID:      "test",
				Result:  json.RawMessage(`{"status":"ok","data":[1,2,3]}`),
			},
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			input:   `{"invalid": json}` + "\n",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &mockReadWriteCloser{
				readData: tt.input,
			}
			session := &jsonrps.Session{
				Conn: conn,
			}

			response, err := session.ReadResponse()

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if response == nil {
				t.Fatal("Expected response but got nil")
			}

			if response.Version != tt.expected.Version {
				t.Errorf("Expected Version %q, got %q", tt.expected.Version, response.Version)
			}

			if !reflect.DeepEqual(response.ID, tt.expected.ID) {
				t.Errorf("Expected ID %v, got %v", tt.expected.ID, response.ID)
			}

			if len(tt.expected.Result) > 0 {
				if !bytes.Equal(response.Result, tt.expected.Result) {
					t.Errorf("Expected Result %s, got %s", tt.expected.Result, response.Result)
				}
			}

			if tt.expected.Error != nil {
				if response.Error == nil {
					t.Error("Expected error object but got nil")
				} else {
					if response.Error.Code != tt.expected.Error.Code {
						t.Errorf("Expected error code %d, got %d", tt.expected.Error.Code, response.Error.Code)
					}
					if response.Error.Message != tt.expected.Error.Message {
						t.Errorf("Expected error message %q, got %q", tt.expected.Error.Message, response.Error.Message)
					}
				}
			}
		})
	}
}

func TestSession_ReadResponse_Error(t *testing.T) {
	// Test read error
	conn := &mockReadWriteCloser{
		readData: "", // Empty data causes EOF immediately
	}
	session := &jsonrps.Session{
		Conn: conn,
	}

	response, err := session.ReadResponse()
	if err == nil {
		t.Error("Expected error when reading from connection with no data")
	}

	if response != nil {
		t.Error("Expected nil response when error occurs")
	}
}

func TestSession_JSONRPCMethods_Integration(t *testing.T) {
	// Test round-trip: write request -> read request, write response -> read response

	// Original request
	originalRequest := &jsonrps.JSONRPCRequest{
		Version: "2.0",
		Method:  "test.echo",
		Params:  json.RawMessage(`{"message":"hello"}`),
		ID:      "integration-test",
	}

	// Step 1: Client writes request
	clientConn := &mockReadWriteCloser{}
	clientSession := &jsonrps.Session{Conn: clientConn}

	err := clientSession.WriteRequest(originalRequest)
	if err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	// Get the written request data
	requestJSON := clientConn.writeData.String()
	requestJSON = strings.TrimPrefix(requestJSON, "\r\n") // Remove header prefix

	// Step 2: Server reads the request
	serverConn := &mockReadWriteCloser{
		readData: requestJSON,
	}
	serverSession := &jsonrps.Session{Conn: serverConn}

	receivedRequest, err := serverSession.ReadRequest()
	if err != nil {
		t.Fatalf("Failed to read request: %v", err)
	}

	// Verify request was transmitted correctly
	if receivedRequest.Version != originalRequest.Version {
		t.Errorf("Request version mismatch: expected %q, got %q", originalRequest.Version, receivedRequest.Version)
	}
	if receivedRequest.Method != originalRequest.Method {
		t.Errorf("Request method mismatch: expected %q, got %q", originalRequest.Method, receivedRequest.Method)
	}
	if !bytes.Equal(receivedRequest.Params, originalRequest.Params) {
		t.Errorf("Request params mismatch: expected %s, got %s", originalRequest.Params, receivedRequest.Params)
	}

	// Step 3: Server writes response
	response := &jsonrps.JSONRPCResponse{
		Version: "2.0",
		ID:      receivedRequest.ID,
		Result:  json.RawMessage(`{"echo":"hello"}`),
	}

	err = serverSession.WriteResponse(response)
	if err != nil {
		t.Fatalf("Failed to write response: %v", err)
	}

	// Get the written response data
	responseJSON := serverConn.writeData.String()
	responseJSON = strings.TrimPrefix(responseJSON, "\r\n") // Remove header prefix

	// Step 4: Client reads response
	clientConn2 := &mockReadWriteCloser{
		readData: responseJSON,
	}
	clientSession2 := &jsonrps.Session{Conn: clientConn2}

	receivedResponse, err := clientSession2.ReadResponse()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	// Verify response was transmitted correctly
	if receivedResponse.Version != response.Version {
		t.Errorf("Response version mismatch: expected %q, got %q", response.Version, receivedResponse.Version)
	}
	if !reflect.DeepEqual(receivedResponse.ID, response.ID) {
		t.Errorf("Response ID mismatch: expected %v, got %v", response.ID, receivedResponse.ID)
	}
	if !bytes.Equal(receivedResponse.Result, response.Result) {
		t.Errorf("Response result mismatch: expected %s, got %s", response.Result, receivedResponse.Result)
	}
}
