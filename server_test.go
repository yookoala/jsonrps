package jsonrps_test

import (
	"bytes"
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
	// Test that a 200 OK response is written for successful header parsing
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

	// Give the goroutine time to execute
	time.Sleep(10 * time.Millisecond)

	// Verify that a 200 OK response was written
	expectedResponse := jsonrps.DefaultProtocolSignature + " 200 OK\r\n\r\n"
	writtenData := mockConn.GetWritten()
	if writtenData != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, writtenData)
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

	// Give the goroutine time to execute and verify 200 OK response
	time.Sleep(10 * time.Millisecond)
	expectedResponse := jsonrps.DefaultProtocolSignature + " 200 OK\r\n\r\n"
	writtenData := mockConn.GetWritten()
	if writtenData != expectedResponse {
		t.Errorf("Expected response %q, got %q", expectedResponse, writtenData)
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
