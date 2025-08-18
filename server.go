package jsonrps

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
)

const (
	// Default protocol signature
	DefaultProtocolSignature = "JSONRPS/1.0"

	// Default MIME type of the RPC + PubSub content
	DefaultMimeType = "application/json+rps"
)

// InitializeServerSession provides default connection handling logic of server.
func InitializeServerSession(c net.Conn) (sess *Session, err error) {
	s := &Session{
		ProtocolSignature: DefaultProtocolSignature,
		Conn:              c,
		Headers:           make(http.Header),
	}

	// Read each line as if it is HTTP header into sess.Headers
	// and stop when reaching "\n\n"
	var line string
	for {
		line, err = bufio.NewReader(c).ReadString('\n')
		if err != nil {
			break
		}
		if line == "\n" {
			break
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) < 2 {
			c.Write([]byte(DefaultProtocolSignature + " 400 Bad Request\r\n\r\n"))
			c.Close()
			return
		}
		if len(parts) == 2 {
			s.Headers.Add(parts[0], strings.TrimSpace(parts[1]))
		}
	}

	sess = s
	return
}

// Method is a function type that handles JSON-RPC requests.
type Method func(ctx context.Context, request *JSONRPCRequest) (response *JSONRPCResponse, err error)

// Server is the interface for the JSON-RPC+PubSub stream servers.
type Server interface {
	ServerSessionHandler

	SetMethod(name string, method Method)
}

// NewServer creates a new instance of default implementation
// of ServerSessionHandler.
func NewServer() Server {
	return &defaultServer{
		mimeType:    DefaultMimeType,
		methods:     make(map[string]Method),
		methodsLock: sync.Mutex{},
	}
}

// defaultServer is the default implementation of ServerSessionHandler.
type defaultServer struct {
	mimeType string

	methods     map[string]Method
	methodsLock sync.Mutex
}

// CanHandleSession checks if the server can handle the given session.
func (h *defaultServer) CanHandleSession(session *Session) bool {
	return session.Headers.Get("Accept") == h.mimeType
}

// HandleSession handles the incoming session.
func (h *defaultServer) HandleSession(session *Session) {
	// Handle the session based on the MIME type
}

// SetMethod sets a method for the given name.
func (h *defaultServer) SetMethod(name string, method Method) {
	h.methodsLock.Lock()
	defer h.methodsLock.Unlock()
	if method == nil {
		delete(h.methods, name)
	} else {
		h.methods[name] = method
	}
}

// NotImplementedServerSessionHandler is an implementation of ServerSessionHandler
// that always returns a "Not Implemented" error.
type NotImplementedServerSessionHandler int

func (h NotImplementedServerSessionHandler) CanHandleSession(session *Session) bool {
	return true
}

func (h NotImplementedServerSessionHandler) HandleSession(session *Session) {
	// No-op handler returns error and close the session
	session.Conn.Write([]byte(DefaultProtocolSignature + " 501 Not Implemented\r\n\r\n"))
	session.Conn.Close()
}
