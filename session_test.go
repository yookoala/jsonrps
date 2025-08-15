package jsonrps_test

import (
	"context"
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
