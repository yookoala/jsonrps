package jsonrps_test

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/yookoala/jsonrps"
)

// MockClientConnection implements net.Conn for testing client functionality
type MockClientConnection struct {
	data          []byte
	position      int
	writeBuffer   *bytes.Buffer
	writeBufferMu sync.Mutex
	closeCallback func()
	closed        bool
}

func NewMockClientConnection(data string) *MockClientConnection {
	return &MockClientConnection{
		data:        []byte(data),
		position:    0,
		writeBuffer: bytes.NewBuffer(nil),
	}
}

func (m *MockClientConnection) Read(b []byte) (n int, err error) {
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

func (m *MockClientConnection) Write(b []byte) (n int, err error) {
	m.writeBufferMu.Lock()
	defer m.writeBufferMu.Unlock()
	return m.writeBuffer.Write(b)
}

func (m *MockClientConnection) Close() error {
	m.closed = true
	if m.closeCallback != nil {
		m.closeCallback()
	}
	return nil
}

func (m *MockClientConnection) GetWritten() string {
	m.writeBufferMu.Lock()
	defer m.writeBufferMu.Unlock()
	return m.writeBuffer.String()
}

func (m *MockClientConnection) LocalAddr() net.Addr                { return nil }
func (m *MockClientConnection) RemoteAddr() net.Addr               { return nil }
func (m *MockClientConnection) SetDeadline(t time.Time) error      { return nil }
func (m *MockClientConnection) SetReadDeadline(t time.Time) error  { return nil }
func (m *MockClientConnection) SetWriteDeadline(t time.Time) error { return nil }

func TestDefaultClientHeader(t *testing.T) {
	// Test that DefaultClientHeader returns expected headers
	headers := jsonrps.DefaultClientHeader()

	// Verify headers is not nil
	if headers == nil {
		t.Fatal("Expected headers to be non-nil")
	}

	// Verify Accept header is set correctly
	acceptHeader := headers.Get("Accept")
	expectedAccept := "application/json+rps"
	if acceptHeader != expectedAccept {
		t.Errorf("Expected Accept header to be %q, got %q", expectedAccept, acceptHeader)
	}

	// Verify no other headers are set by default
	if len(headers) != 1 {
		t.Errorf("Expected exactly 1 header to be set, got %d", len(headers))
	}
}

func TestDefaultClientHeader_Modifiable(t *testing.T) {
	// Test that the returned header can be modified
	headers := jsonrps.DefaultClientHeader()

	// Add a custom header
	headers.Add("User-Agent", "Test Client 1.0")

	// Verify custom header was added
	userAgent := headers.Get("User-Agent")
	if userAgent != "Test Client 1.0" {
		t.Errorf("Expected User-Agent to be 'Test Client 1.0', got %q", userAgent)
	}

	// Verify Accept header is still there
	acceptHeader := headers.Get("Accept")
	if acceptHeader != "application/json+rps" {
		t.Errorf("Expected Accept header to still be 'application/json+rps', got %q", acceptHeader)
	}
}

func TestDefaultClientHeader_Independent(t *testing.T) {
	// Test that multiple calls return independent header objects
	headers1 := jsonrps.DefaultClientHeader()
	headers2 := jsonrps.DefaultClientHeader()

	// Modify one of them
	headers1.Add("Custom-Header", "value1")

	// Verify the other is not affected
	customHeader := headers2.Get("Custom-Header")
	if customHeader != "" {
		t.Errorf("Expected headers2 to not have Custom-Header, got %q", customHeader)
	}
}

func TestInitializeClientConn_BasicFunctionality(t *testing.T) {
	// Test basic client connection initialization
	responseData := "\r\n" // Empty headers response

	mockConn := NewMockClientConnection(responseData)
	headers := jsonrps.DefaultClientHeader()
	logger := createTestLogger(t)

	session, err := jsonrps.InitializeClientSession(mockConn, headers, logger)

	// Give the goroutine time to write headers
	time.Sleep(10 * time.Millisecond)

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

	// Verify connection is set
	if session.Conn != mockConn {
		t.Error("Expected session.Conn to be the mock connection")
	}

	// Verify headers were written to connection
	writtenData := mockConn.GetWritten()
	expectedHeaders := "Accept: application/json+rps\r\n\r\n"
	if writtenData != expectedHeaders {
		t.Errorf("Expected headers %q to be written, got %q", expectedHeaders, writtenData)
	}
}

func TestInitializeClientConn_WithCustomHeaders(t *testing.T) {
	// Test client connection with custom headers
	responseData := "\r\n"

	mockConn := NewMockClientConnection(responseData)
	headers := make(http.Header)
	headers.Add("Accept", "application/json")
	headers.Add("User-Agent", "Test Client 1.0")
	headers.Add("Authorization", "Bearer token123")
	logger := createTestLogger(t)

	session, err := jsonrps.InitializeClientSession(mockConn, headers, logger)

	// Give the goroutine time to write headers
	time.Sleep(10 * time.Millisecond)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify session was returned
	if session == nil {
		t.Fatal("Expected session to be returned")
	}

	// Verify headers were written to connection
	writtenData := mockConn.GetWritten()

	// Check that all expected headers are present in the written data
	expectedStrings := []string{
		"Accept: application/json\r\n",
		"User-Agent: Test Client 1.0\r\n",
		"Authorization: Bearer token123\r\n",
		"\r\n", // Final CRLF
	}

	for _, expected := range expectedStrings {
		if !containsString(writtenData, expected) {
			t.Errorf("Expected written data to contain %q, got %q", expected, writtenData)
		}
	}
}

func TestInitializeClientConn_WithResponseHeaders(t *testing.T) {
	// Test client connection that receives response headers
	responseData := "Content-Type: application/json\r\n" +
		"Server: Test Server 1.0\r\n" +
		"\r\n"

	mockConn := NewMockClientConnection(responseData)
	headers := jsonrps.DefaultClientHeader()
	logger := createTestLogger(t)

	session, err := jsonrps.InitializeClientSession(mockConn, headers, logger)

	// Give the goroutine time to write headers
	time.Sleep(10 * time.Millisecond)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify session was returned
	if session == nil {
		t.Fatal("Expected session to be returned")
	}

	// Verify response headers were parsed into session
	contentType := session.Headers.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type to be 'application/json', got %q", contentType)
	}

	server := session.Headers.Get("Server")
	if server != "Test Server 1.0" {
		t.Errorf("Expected Server to be 'Test Server 1.0', got %q", server)
	}
}

func TestInitializeClientConn_EmptyHeaders(t *testing.T) {
	// Test client connection with empty headers
	responseData := "\r\n"

	mockConn := NewMockClientConnection(responseData)
	headers := make(http.Header) // Empty headers
	logger := createTestLogger(t)

	session, err := jsonrps.InitializeClientSession(mockConn, headers, logger)

	// Give the goroutine time to write headers
	time.Sleep(10 * time.Millisecond)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify session was returned
	if session == nil {
		t.Fatal("Expected session to be returned")
	}

	// Verify only the final CRLF was written
	writtenData := mockConn.GetWritten()
	if writtenData != "\r\n" {
		t.Errorf("Expected only final CRLF to be written, got %q", writtenData)
	}
}

func TestInitializeClientConn_ReadError(t *testing.T) {
	// Test client connection with read error
	mockConn := &MockClientConnection{
		data:        []byte{}, // Empty data will cause EOF
		position:    0,
		writeBuffer: bytes.NewBuffer(nil),
	}

	headers := jsonrps.DefaultClientHeader()
	logger := createTestLogger(t)

	session, err := jsonrps.InitializeClientSession(mockConn, headers, logger)

	// Give the goroutine time to write headers
	time.Sleep(10 * time.Millisecond)

	// Verify no error occurred (function handles read errors gracefully)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify session was returned
	if session == nil {
		t.Fatal("Expected session to be returned")
	}

	// Headers should be empty due to read error
	if len(session.Headers) != 0 {
		t.Errorf("Expected no headers due to read error, got %d", len(session.Headers))
	}
}

func TestInitializeClientConn_HeadersWithSpaces(t *testing.T) {
	// Test parsing of response headers with spaces in values
	responseData := "Content-Type: application/json; charset=utf-8\r\n" +
		"Cache-Control: no-cache, no-store\r\n" +
		"\r\n"

	mockConn := NewMockClientConnection(responseData)
	headers := jsonrps.DefaultClientHeader()
	logger := createTestLogger(t)

	session, err := jsonrps.InitializeClientSession(mockConn, headers, logger)

	// Give the goroutine time to write headers
	time.Sleep(10 * time.Millisecond)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify headers with spaces in values are parsed correctly
	contentType := session.Headers.Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("Expected Content-Type to be 'application/json; charset=utf-8', got %q", contentType)
	}

	cacheControl := session.Headers.Get("Cache-Control")
	if cacheControl != "no-cache, no-store" {
		t.Errorf("Expected Cache-Control to be 'no-cache, no-store', got %q", cacheControl)
	}
}

func TestInitializeClientConn_HeadersWithTrailingSpaces(t *testing.T) {
	// Test that trailing spaces in header values are trimmed
	responseData := "Content-Type: application/json   \r\n" +
		"Server: Test Server   \r\n" +
		"\r\n"

	mockConn := NewMockClientConnection(responseData)
	headers := jsonrps.DefaultClientHeader()
	logger := createTestLogger(t)

	session, err := jsonrps.InitializeClientSession(mockConn, headers, logger)

	// Give the goroutine time to write headers
	time.Sleep(10 * time.Millisecond)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify trailing spaces are trimmed
	contentType := session.Headers.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type to be 'application/json' (trimmed), got %q", contentType)
	}

	server := session.Headers.Get("Server")
	if server != "Test Server" {
		t.Errorf("Expected Server to be 'Test Server' (trimmed), got %q", server)
	}
}

func TestInitializeClientConn_MultipleHeaderValues(t *testing.T) {
	// Test client connection writing multiple values for same header
	responseData := "\r\n"

	mockConn := NewMockClientConnection(responseData)
	headers := make(http.Header)
	headers.Add("Accept", "application/json")
	headers.Add("Accept", "application/xml")
	logger := createTestLogger(t)

	session, err := jsonrps.InitializeClientSession(mockConn, headers, logger)

	// Give the goroutine time to write headers
	time.Sleep(10 * time.Millisecond)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify session was returned
	if session == nil {
		t.Fatal("Expected session to be returned")
	}

	// Verify both header values were written
	writtenData := mockConn.GetWritten()
	if !containsString(writtenData, "Accept: application/json\r\n") {
		t.Error("Expected first Accept header to be written")
	}
	if !containsString(writtenData, "Accept: application/xml\r\n") {
		t.Error("Expected second Accept header to be written")
	}
}

func TestInitializeClientConn_AsyncHeaderWriting(t *testing.T) {
	// Test that headers are written asynchronously
	responseData := "\r\n"

	mockConn := NewMockClientConnection(responseData)
	headers := jsonrps.DefaultClientHeader()
	logger := createTestLogger(t)

	// Execute the function
	session, err := jsonrps.InitializeClientSession(mockConn, headers, logger)

	// Verify no error occurred
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify session was returned immediately
	if session == nil {
		t.Fatal("Expected session to be returned immediately")
	}

	// The function returns immediately while headers are written in background
	// We test that by ensuring headers are eventually written
	time.Sleep(10 * time.Millisecond)
	writtenData := mockConn.GetWritten()
	expectedHeaders := "Accept: application/json+rps\r\n\r\n"
	if writtenData != expectedHeaders {
		t.Errorf("Expected headers %q to be written eventually, got %q", expectedHeaders, writtenData)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
