package jsonrps_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/yookoala/jsonrps"
)

// MockConnection implements net.Conn for testing
// This mock simulates the behavior of reading one byte at a time to work
// with the bufio.NewReader pattern used in HandleServerConn
type MockConnection struct {
	data          []byte
	position      int
	writeBuffer   *bytes.Buffer
	writeBufferMu sync.Mutex
	closeCallback func()
	closed        bool
}

func NewMockConnection(data string) *MockConnection {
	return &MockConnection{
		data:        []byte(data),
		position:    0,
		writeBuffer: bytes.NewBuffer(nil),
	}
}

func (m *MockConnection) Read(b []byte) (n int, err error) {
	if m.position >= len(m.data) {
		return 0, io.EOF
	}

	// Read one byte at a time to simulate real network behavior
	if len(b) > 0 {
		b[0] = m.data[m.position]
		m.position++
		return 1, nil
	}
	return 0, nil
}

func (m *MockConnection) Write(b []byte) (n int, err error) {
	m.writeBufferMu.Lock()
	defer m.writeBufferMu.Unlock()
	return m.writeBuffer.Write(b)
}

func (m *MockConnection) Close() error {
	m.closed = true
	if m.closeCallback != nil {
		m.closeCallback()
	}
	return nil
}

func (m *MockConnection) GetWritten() string {
	m.writeBufferMu.Lock()
	defer m.writeBufferMu.Unlock()
	return m.writeBuffer.String()
}

func (m *MockConnection) LocalAddr() net.Addr                { return nil }
func (m *MockConnection) RemoteAddr() net.Addr               { return nil }
func (m *MockConnection) SetDeadline(t time.Time) error      { return nil }
func (m *MockConnection) SetReadDeadline(t time.Time) error  { return nil }
func (m *MockConnection) SetWriteDeadline(t time.Time) error { return nil }

// MockSessionHandler implements ServerSessionHandler for testing
type MockSessionHandler struct {
	canHandle      bool
	sessionHandled *jsonrps.Session
	handleCallback func(*jsonrps.Session)
}

func (m *MockSessionHandler) CanHandleSession(session *jsonrps.Session) bool {
	return m.canHandle
}

func (m *MockSessionHandler) HandleSession(session *jsonrps.Session) {
	m.sessionHandled = session
	if m.handleCallback != nil {
		m.handleCallback(session)
	}
}

func TestDefaultProtocolSignature(t *testing.T) {
	expected := "JSONRPS/1.0"
	if jsonrps.DefaultProtocolSignature != expected {
		t.Errorf("Expected DefaultProtocolSignature to be %q, got %q", expected, jsonrps.DefaultProtocolSignature)
	}
}

func TestInitializeServerConn_ValidHeaders(t *testing.T) {
	// Prepare test data with valid headers
	testData := "Content-Type: application/json\r\n" +
		"Authorization: Bearer token123\r\n" +
		"Custom-Header: test-value\r\n" +
		"\n"

	mockConn := NewMockConnection(testData)

	// Execute the function
	session, err := jsonrps.InitializeServerSession(mockConn)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify session was returned
	if session == nil {
		t.Fatal("Expected session to be returned")
	}

	// Verify protocol signature
	if session.ProtocolSignature != jsonrps.DefaultProtocolSignature {
		t.Errorf("Expected ProtocolSignature to be %q, got %q", jsonrps.DefaultProtocolSignature, session.ProtocolSignature)
	}

	// Verify connection
	if session.Conn != mockConn {
		t.Error("Expected session.Conn to be the mock connection")
	}

	// Verify headers
	expectedHeaders := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token123",
		"Custom-Header": "test-value",
	}

	for key, expectedValue := range expectedHeaders {
		actualValue := session.Headers.Get(key)
		if actualValue != expectedValue {
			t.Errorf("Expected header %q to be %q, got %q", key, expectedValue, actualValue)
		}
	}
}

func TestInitializeServerConn_SuccessResponse(t *testing.T) {
	// Test that InitializeServerSession parses headers successfully without sending automatic response
	testData := "Content-Type: application/json\r\n\n"

	mockConn := NewMockConnection(testData)

	// Execute the function
	session, err := jsonrps.InitializeServerSession(mockConn)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify session was returned
	if session == nil {
		t.Fatal("Expected session to be returned")
	}

	// Verify headers were parsed correctly
	if session.Headers.Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", session.Headers.Get("Content-Type"))
	}

	// Verify that NO response was automatically written by InitializeServerSession
	writtenData := mockConn.GetWritten()
	if writtenData != "" {
		t.Errorf("Expected no automatic response from InitializeServerSession, got %q", writtenData)
	}
}

func TestInitializeServerConn_EmptyHeaders(t *testing.T) {
	// Test with just the terminating newline
	testData := "\n"

	mockConn := NewMockConnection(testData)

	session, err := jsonrps.InitializeServerSession(mockConn)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify session was returned
	if session == nil {
		t.Fatal("Expected session to be returned")
	}

	// Verify headers is empty but initialized
	if session.Headers == nil {
		t.Error("Expected session.Headers to be initialized")
	}

	if len(session.Headers) != 0 {
		t.Errorf("Expected no headers, got %d", len(session.Headers))
	}

	// Verify that NO response was automatically written by InitializeServerSession
	writtenData := mockConn.GetWritten()
	if writtenData != "" {
		t.Errorf("Expected no automatic response from InitializeServerSession, got %q", writtenData)
	}
}

func TestInitializeServerConn_MalformedHeader(t *testing.T) {
	// Test with a malformed header (no colon and space)
	testData := "InvalidHeaderLine\r\n\n"

	mockConn := NewMockConnection(testData)

	session, _ := jsonrps.InitializeServerSession(mockConn)

	// Verify that a 400 Bad Request was written
	expectedResponse := jsonrps.DefaultProtocolSignature + " 400 Bad Request\r\n\r\n"
	writtenData := mockConn.GetWritten()
	if writtenData != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, writtenData)
	}

	// Wait a bit to ensure no 200 OK response is written by a goroutine
	time.Sleep(10 * time.Millisecond)
	writtenDataAfterWait := mockConn.GetWritten()
	if writtenDataAfterWait != expectedResponse {
		t.Errorf("Expected response to remain %q after wait, got %q", expectedResponse, writtenDataAfterWait)
	}

	// Verify connection was closed
	if !mockConn.closed {
		t.Error("Expected connection to be closed after malformed header")
	}

	// Verify session was not returned (should be nil) - the function returns early
	if session != nil {
		t.Errorf("Expected session to be nil due to malformed header, got session with signature: %q", session.ProtocolSignature)
	}

	// Note: err might be nil if the function returns early without setting it,
	// but the 400 response and connection closure indicate the error condition
}

func TestInitializeServerConn_HeaderWithSpacesInValue(t *testing.T) {
	// Test with headers that have spaces in values
	testData := "Content-Type: application/json; charset=utf-8\r\n" +
		"User-Agent: Test Agent 1.0\r\n" +
		"\n"

	mockConn := NewMockConnection(testData)

	session, err := jsonrps.InitializeServerSession(mockConn)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify session was returned
	if session == nil {
		t.Fatal("Expected session to be returned")
	}

	// Verify headers with spaces in values are parsed correctly
	contentType := session.Headers.Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("Expected Content-Type to be 'application/json; charset=utf-8', got %q", contentType)
	}

	userAgent := session.Headers.Get("User-Agent")
	if userAgent != "Test Agent 1.0" {
		t.Errorf("Expected User-Agent to be 'Test Agent 1.0', got %q", userAgent)
	}
}

func TestInitializeServerConn_ReadError(t *testing.T) {
	// Create a connection that will cause a read error
	mockConn := &MockConnection{
		data:        []byte{}, // Empty data will cause EOF
		position:    0,
		writeBuffer: bytes.NewBuffer(nil),
	}

	session, err := jsonrps.InitializeServerSession(mockConn)

	// The function should handle the read error gracefully
	// In this case, err should be set or session might be nil
	if err == nil && session == nil {
		t.Fatal("Expected either error or session to be returned")
	}

	// If session is returned, headers should be empty due to read error
	if session != nil && len(session.Headers) != 0 {
		t.Errorf("Expected no headers due to read error, got %d", len(session.Headers))
	}
}

func TestInitializeServerConn_HeaderWithoutSpace(t *testing.T) {
	// Test with a header that has a colon but no space after it
	testData := "Content-Type:application/json\r\n\n"

	mockConn := NewMockConnection(testData)

	session, _ := jsonrps.InitializeServerSession(mockConn)

	// This should be treated as malformed and result in a 400 response
	expectedResponse := jsonrps.DefaultProtocolSignature + " 400 Bad Request\r\n\r\n"
	writtenData := mockConn.GetWritten()
	if writtenData != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, writtenData)
	}

	// Verify connection was closed
	if !mockConn.closed {
		t.Error("Expected connection to be closed after malformed header")
	}

	// Verify session was not returned due to malformed header
	if session != nil {
		t.Error("Expected session to be nil due to malformed header")
	}
}

func TestInitializeServerConn_HeaderWithTrailingSpaces(t *testing.T) {
	// Test with headers that have trailing spaces and carriage returns
	testData := "Content-Type: application/json   \r\n" +
		"Authorization: Bearer token   \r\n" +
		"\n"

	mockConn := NewMockConnection(testData)

	session, err := jsonrps.InitializeServerSession(mockConn)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify session was returned
	if session == nil {
		t.Fatal("Expected session to be returned")
	}

	// Verify trailing spaces are trimmed
	contentType := session.Headers.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type to be 'application/json' (trimmed), got %q", contentType)
	}

	auth := session.Headers.Get("Authorization")
	if auth != "Bearer token" {
		t.Errorf("Expected Authorization to be 'Bearer token' (trimmed), got %q", auth)
	}
}

func TestNewServer(t *testing.T) {
	// Test that NewServer returns a valid server
	server := jsonrps.NewServer()

	if server == nil {
		t.Fatal("Expected NewServer to return a non-nil server")
	}

	// Test with a session that has the default MIME type in Accept header
	session := &jsonrps.Session{
		Headers: make(map[string][]string),
	}
	session.Headers.Set("Accept", jsonrps.DefaultMimeType)

	canHandle := server.CanHandleSession(session)
	if !canHandle {
		t.Error("Expected default server to handle session with default MIME type in Accept header")
	}
}

func TestNewServer_Integration(t *testing.T) {
	// Test the full integration of NewServer
	testData := "Accept: " + jsonrps.DefaultMimeType + "\r\n\n"
	mockConn := NewMockConnection(testData)

	session, err := jsonrps.InitializeServerSession(mockConn)
	if err != nil {
		t.Fatalf("Expected no error from InitializeServerSession, got %v", err)
	}

	server := jsonrps.NewServer()

	// Should be able to handle the session
	if !server.CanHandleSession(session) {
		t.Error("Expected new server to be able to handle session with default MIME type in Accept header")
	}

	// Handle the session (should not panic)
	server.HandleSession(session)
}

func TestNewServer_HandlingBehavior(t *testing.T) {
	tests := []struct {
		name       string
		acceptType string
		expected   bool
	}{
		{
			name:       "default MIME type",
			acceptType: jsonrps.DefaultMimeType,
			expected:   true,
		},
		{
			name:       "JSON MIME type",
			acceptType: "application/json",
			expected:   false, // defaultServer only handles default MIME type
		},
		{
			name:       "text plain",
			acceptType: "text/plain",
			expected:   false, // defaultServer only handles default MIME type
		},
		{
			name:       "empty accept type",
			acceptType: "",
			expected:   false, // defaultServer only handles default MIME type
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := jsonrps.NewServer()

			session := &jsonrps.Session{
				Headers: make(map[string][]string),
			}
			if tt.acceptType != "" {
				session.Headers.Set("Accept", tt.acceptType)
			}

			result := server.CanHandleSession(session)
			if result != tt.expected {
				t.Errorf("Expected CanHandleSession to return %v for accept type %q, got %v",
					tt.expected, tt.acceptType, result)
			}
		})
	}
}

func TestNewServer_DetailedHandling(t *testing.T) {
	// Test that the server handles different MIME types appropriately
	server := jsonrps.NewServer()

	// Test default MIME type in Accept header - should be handled by defaultServer
	session1 := &jsonrps.Session{
		Headers: make(map[string][]string),
		Conn:    NewMockConnection(""),
	}
	session1.Headers.Set("Accept", jsonrps.DefaultMimeType)

	if !server.CanHandleSession(session1) {
		t.Error("Expected server to handle session with default MIME type")
	}

	server.HandleSession(session1)

	// Should not get a 501 response since it's handled by defaultServer
	written1 := session1.Conn.(*MockConnection).GetWritten()
	notImplementedResponse := jsonrps.DefaultProtocolSignature + " 501 Not Implemented\r\n\r\n"
	if written1 == notImplementedResponse {
		t.Error("Expected default MIME type to be handled by defaultServer, not with 501 response")
	}

	// Test unknown MIME type in Accept header - should not be handled by defaultServer
	session2 := &jsonrps.Session{
		Headers: make(map[string][]string),
		Conn:    NewMockConnection(""),
	}
	session2.Headers.Set("Accept", "unknown/type")

	if server.CanHandleSession(session2) {
		t.Error("Expected server to not handle session with unknown MIME type")
	}
}

func TestDefaultServer_HandleSession(t *testing.T) {
	// Test that HandleSession doesn't panic and can be called
	server := jsonrps.NewServer()
	mockConn := NewMockConnection("")

	session := &jsonrps.Session{
		Headers: make(map[string][]string),
		Conn:    mockConn,
	}
	session.Headers.Set("Accept", jsonrps.DefaultMimeType)

	// Should not panic
	server.HandleSession(session)
}

func TestDefaultServer_SetMethod(t *testing.T) {
	// Test the SetMethod functionality of the Server
	server := jsonrps.NewServer()

	// Test setting a method
	testMethod := func(ctx context.Context, request *jsonrps.JSONRPCRequest) (*jsonrps.JSONRPCResponse, error) {
		return &jsonrps.JSONRPCResponse{
			ID:     request.ID,
			Result: json.RawMessage(`"test result"`),
		}, nil
	}

	// Should not panic when setting a method
	server.SetMethod("test.method", testMethod)

	// Test setting method to nil (removal)
	server.SetMethod("test.method", nil)

	// Test setting multiple methods
	server.SetMethod("method1", testMethod)
	server.SetMethod("method2", testMethod)
	server.SetMethod("method3", testMethod)

	// Remove one method
	server.SetMethod("method2", nil)
}

func TestNotImplementedServerSessionHandler_CanHandleSession(t *testing.T) {
	// Test that NotImplementedServerSessionHandler always returns true for CanHandleSession
	handler := jsonrps.NotImplementedServerSessionHandler(0)

	tests := []struct {
		name    string
		session *jsonrps.Session
	}{
		{
			name: "session with headers",
			session: &jsonrps.Session{
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
			},
		},
		{
			name: "session without headers",
			session: &jsonrps.Session{
				Headers: make(map[string][]string),
			},
		},
		{
			name: "session with nil headers",
			session: &jsonrps.Session{
				Headers: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.CanHandleSession(tt.session)
			if !result {
				t.Error("Expected NotImplementedServerSessionHandler.CanHandleSession to always return true")
			}
		})
	}
}

func TestNotImplementedServerSessionHandler_HandleSession(t *testing.T) {
	// Test that NotImplementedServerSessionHandler sends 501 response and closes connection
	handler := jsonrps.NotImplementedServerSessionHandler(0)
	mockConn := NewMockConnection("")

	session := &jsonrps.Session{
		Headers: make(map[string][]string),
		Conn:    mockConn,
	}

	// Handle the session
	handler.HandleSession(session)

	// Verify 501 Not Implemented was written
	expectedResponse := jsonrps.DefaultProtocolSignature + " 501 Not Implemented\r\n\r\n"
	writtenData := mockConn.GetWritten()
	if writtenData != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, writtenData)
	}

	// Verify connection was closed
	if !mockConn.closed {
		t.Error("Expected connection to be closed after NotImplementedServerSessionHandler.HandleSession")
	}
}

func TestNotImplementedServerSessionHandler_DifferentValues(t *testing.T) {
	// Test that different values of NotImplementedServerSessionHandler behave the same
	handler1 := jsonrps.NotImplementedServerSessionHandler(0)
	handler2 := jsonrps.NotImplementedServerSessionHandler(42)
	handler3 := jsonrps.NotImplementedServerSessionHandler(-1)

	session := &jsonrps.Session{
		Headers: make(map[string][]string),
	}

	// All should return true for CanHandleSession
	if !handler1.CanHandleSession(session) {
		t.Error("Expected handler1.CanHandleSession to return true")
	}
	if !handler2.CanHandleSession(session) {
		t.Error("Expected handler2.CanHandleSession to return true")
	}
	if !handler3.CanHandleSession(session) {
		t.Error("Expected handler3.CanHandleSession to return true")
	}

	// Test HandleSession for each
	mockConn1 := NewMockConnection("")
	mockConn2 := NewMockConnection("")
	mockConn3 := NewMockConnection("")

	session1 := &jsonrps.Session{Headers: make(map[string][]string), Conn: mockConn1}
	session2 := &jsonrps.Session{Headers: make(map[string][]string), Conn: mockConn2}
	session3 := &jsonrps.Session{Headers: make(map[string][]string), Conn: mockConn3}

	handler1.HandleSession(session1)
	handler2.HandleSession(session2)
	handler3.HandleSession(session3)

	// All should send the same response
	expectedResponse := jsonrps.DefaultProtocolSignature + " 501 Not Implemented\r\n\r\n"

	if mockConn1.GetWritten() != expectedResponse {
		t.Error("Expected handler1 to send 501 response")
	}
	if mockConn2.GetWritten() != expectedResponse {
		t.Error("Expected handler2 to send 501 response")
	}
	if mockConn3.GetWritten() != expectedResponse {
		t.Error("Expected handler3 to send 501 response")
	}

	// All should close connections
	if !mockConn1.closed || !mockConn2.closed || !mockConn3.closed {
		t.Error("Expected all handlers to close connections")
	}
}

func TestServerSessionHandlers_AsRouter(t *testing.T) {
	// Test that the handlers work correctly when used in a router
	handler1 := jsonrps.NotImplementedServerSessionHandler(0)
	handler2 := jsonrps.NewServer()

	router := jsonrps.ServerSessionRouter{handler1, handler2}

	// Test with default MIME type - should match handler2 (NewServer)
	session1 := &jsonrps.Session{
		Headers: make(map[string][]string),
		Conn:    NewMockConnection(""),
	}
	session1.Headers.Set("Accept", jsonrps.DefaultMimeType)

	if !router.CanHandleSession(session1) {
		t.Error("Expected router to handle session with default MIME type")
	}

	// Test with unknown MIME type - should match handler1 (NotImplementedServerSessionHandler)
	session2 := &jsonrps.Session{
		Headers: make(map[string][]string),
		Conn:    NewMockConnection(""),
	}
	session2.Headers.Set("Accept", "unknown/type")

	if !router.CanHandleSession(session2) {
		t.Error("Expected router to handle session with unknown MIME type via NotImplementedServerSessionHandler")
	}

	// Handle the unknown type session - should get 501 response
	router.HandleSession(session2)
	mockConn2 := session2.Conn.(*MockConnection)
	expectedResponse := jsonrps.DefaultProtocolSignature + " 501 Not Implemented\r\n\r\n"
	if mockConn2.GetWritten() != expectedResponse {
		t.Error("Expected router to delegate to NotImplementedServerSessionHandler for unknown MIME type")
	}
}
