package jsonrps_test

import (
	"bytes"
	"io"
	"net"
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

func TestHandleServerConn_ValidHeaders(t *testing.T) {
	// Prepare test data with valid headers
	testData := "Content-Type: application/json\r\n" +
		"Authorization: Bearer token123\r\n" +
		"Custom-Header: test-value\r\n" +
		"\n"

	mockConn := NewMockConnection(testData)
	mockHandler := &MockSessionHandler{canHandle: true}
	router := jsonrps.ServerSessionRouter{mockHandler}

	// Execute the function
	jsonrps.HandleServerConn(mockConn, router)

	// Verify the session was handled
	if mockHandler.sessionHandled == nil {
		t.Fatal("Expected session to be handled")
	}

	session := mockHandler.sessionHandled

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

func TestHandleServerConn_SuccessResponse(t *testing.T) {
	// Test that a 200 OK response is written for successful header parsing
	testData := "Content-Type: application/json\r\n\n"

	mockConn := NewMockConnection(testData)
	mockHandler := &MockSessionHandler{canHandle: true}
	router := jsonrps.ServerSessionRouter{mockHandler}

	// Execute the function
	jsonrps.HandleServerConn(mockConn, router)

	// Give the goroutine time to execute
	time.Sleep(10 * time.Millisecond)

	// Verify that a 200 OK response was written
	expectedResponse := jsonrps.DefaultProtocolSignature + " 200 OK\r\n\r\n"
	writtenData := mockConn.GetWritten()
	if writtenData != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, writtenData)
	}

	// Verify the session was still handled
	if mockHandler.sessionHandled == nil {
		t.Fatal("Expected session to be handled")
	}
}

func TestHandleServerConn_EmptyHeaders(t *testing.T) {
	// Test with just the terminating newline
	testData := "\n"

	mockConn := NewMockConnection(testData)
	mockHandler := &MockSessionHandler{canHandle: true}
	router := jsonrps.ServerSessionRouter{mockHandler}

	jsonrps.HandleServerConn(mockConn, router)

	// Verify the session was handled
	if mockHandler.sessionHandled == nil {
		t.Fatal("Expected session to be handled")
	}

	session := mockHandler.sessionHandled

	// Verify headers is empty but initialized
	if session.Headers == nil {
		t.Error("Expected session.Headers to be initialized")
	}

	if len(session.Headers) != 0 {
		t.Errorf("Expected no headers, got %d", len(session.Headers))
	}

	// Give the goroutine time to execute and verify 200 OK response
	time.Sleep(10 * time.Millisecond)
	expectedResponse := jsonrps.DefaultProtocolSignature + " 200 OK\r\n\r\n"
	writtenData := mockConn.GetWritten()
	if writtenData != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, writtenData)
	}
}

func TestHandleServerConn_MalformedHeader(t *testing.T) {
	// Test with a malformed header (no colon and space)
	testData := "InvalidHeaderLine\r\n\n"

	mockConn := NewMockConnection(testData)
	mockHandler := &MockSessionHandler{canHandle: true}
	router := jsonrps.ServerSessionRouter{mockHandler}

	jsonrps.HandleServerConn(mockConn, router)

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

	// Verify session was not handled
	if mockHandler.sessionHandled != nil {
		t.Error("Expected session not to be handled due to malformed header")
	}
}

func TestHandleServerConn_AsyncResponseBehavior(t *testing.T) {
	// Test that the session is handled before the 200 OK response is written
	testData := "Content-Type: application/json\r\n\n"

	mockConn := NewMockConnection(testData)
	mockHandler := &MockSessionHandler{canHandle: true}
	router := jsonrps.ServerSessionRouter{mockHandler}

	// Execute the function
	jsonrps.HandleServerConn(mockConn, router)

	// The session should be handled immediately (synchronously)
	if mockHandler.sessionHandled == nil {
		t.Fatal("Expected session to be handled immediately")
	}

	// But the response should not be written yet (it's in a goroutine)
	writtenData := mockConn.GetWritten()
	if writtenData != "" {
		t.Errorf("Expected no response to be written immediately, got %q", writtenData)
	}

	// After waiting, the 200 OK response should be written
	time.Sleep(10 * time.Millisecond)
	expectedResponse := jsonrps.DefaultProtocolSignature + " 200 OK\r\n\r\n"
	writtenDataAfterWait := mockConn.GetWritten()
	if writtenDataAfterWait != expectedResponse {
		t.Errorf("Expected response %q after wait, got %q", expectedResponse, writtenDataAfterWait)
	}
}

func TestHandleServerConn_HeaderWithSpacesInValue(t *testing.T) {
	// Test with headers that have spaces in values
	testData := "Content-Type: application/json; charset=utf-8\r\n" +
		"User-Agent: Test Agent 1.0\r\n" +
		"\n"

	mockConn := NewMockConnection(testData)
	mockHandler := &MockSessionHandler{canHandle: true}
	router := jsonrps.ServerSessionRouter{mockHandler}

	jsonrps.HandleServerConn(mockConn, router)

	session := mockHandler.sessionHandled
	if session == nil {
		t.Fatal("Expected session to be handled")
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

func TestHandleServerConn_MultipleHandlers(t *testing.T) {
	testData := "Content-Type: application/json\r\n\n"

	mockConn := NewMockConnection(testData)

	// Create multiple handlers where only the second one can handle
	handler1 := &MockSessionHandler{canHandle: false}
	handler2 := &MockSessionHandler{canHandle: true}
	handler3 := &MockSessionHandler{canHandle: true} // This shouldn't be called

	router := jsonrps.ServerSessionRouter{handler1, handler2, handler3}

	jsonrps.HandleServerConn(mockConn, router)

	// Verify only the second handler was called
	if handler1.sessionHandled != nil {
		t.Error("Expected first handler not to be called")
	}

	if handler2.sessionHandled == nil {
		t.Error("Expected second handler to be called")
	}

	if handler3.sessionHandled != nil {
		t.Error("Expected third handler not to be called (should stop at second)")
	}
}

func TestHandleServerConn_NoHandlerCanHandle(t *testing.T) {
	testData := "Content-Type: application/json\r\n\n"

	mockConn := NewMockConnection(testData)

	// Create handlers that cannot handle the session
	handler1 := &MockSessionHandler{canHandle: false}
	handler2 := &MockSessionHandler{canHandle: false}

	router := jsonrps.ServerSessionRouter{handler1, handler2}

	jsonrps.HandleServerConn(mockConn, router)

	// Verify no handlers were called
	if handler1.sessionHandled != nil {
		t.Error("Expected first handler not to be called")
	}

	if handler2.sessionHandled != nil {
		t.Error("Expected second handler not to be called")
	}
}

func TestHandleServerConn_ReadError(t *testing.T) {
	// Create a connection that will cause a read error
	mockConn := &MockConnection{
		data:        []byte{}, // Empty data will cause EOF
		position:    0,
		writeBuffer: bytes.NewBuffer(nil),
	}

	mockHandler := &MockSessionHandler{canHandle: true}
	router := jsonrps.ServerSessionRouter{mockHandler}

	jsonrps.HandleServerConn(mockConn, router)

	// The function should handle the read error gracefully and still call the router
	if mockHandler.sessionHandled == nil {
		t.Fatal("Expected session to be handled even with read error")
	}

	// Headers should be empty due to read error
	if len(mockHandler.sessionHandled.Headers) != 0 {
		t.Errorf("Expected no headers due to read error, got %d", len(mockHandler.sessionHandled.Headers))
	}
}

func TestHandleServerConn_HeaderWithoutSpace(t *testing.T) {
	// Test with a header that has a colon but no space after it
	testData := "Content-Type:application/json\r\n\n"

	mockConn := NewMockConnection(testData)
	mockHandler := &MockSessionHandler{canHandle: true}
	router := jsonrps.ServerSessionRouter{mockHandler}

	jsonrps.HandleServerConn(mockConn, router)

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
}

func TestHandleServerConn_HeaderWithTrailingSpaces(t *testing.T) {
	// Test with headers that have trailing spaces and carriage returns
	testData := "Content-Type: application/json   \r\n" +
		"Authorization: Bearer token   \r\n" +
		"\n"

	mockConn := NewMockConnection(testData)
	mockHandler := &MockSessionHandler{canHandle: true}
	router := jsonrps.ServerSessionRouter{mockHandler}

	jsonrps.HandleServerConn(mockConn, router)

	session := mockHandler.sessionHandled
	if session == nil {
		t.Fatal("Expected session to be handled")
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
