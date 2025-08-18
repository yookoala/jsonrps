package jsonrps

import (
	"bufio"
	"net"
	"net/http"
	"strings"
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

// NewServerSessionHandler creates a new instance of default implementation
// of ServerSessionHandler.
func NewServerSessionHandler() ServerSessionHandler {
	r := ServerSessionRouter{
		&defaultServerSessionHandler{
			mimeType: DefaultMimeType,
		},
		NotImplementedServerSessionHandler(0),
	}
	return r
}

// defaultServerSessionHandler is the default implementation of ServerSessionHandler.
type defaultServerSessionHandler struct {
	mimeType string
}

func (h defaultServerSessionHandler) CanHandleSession(session *Session) bool {
	return session.Headers.Get("Content-Type") == h.mimeType
}

func (h *defaultServerSessionHandler) HandleSession(session *Session) {
	// Handle the session based on the MIME type
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
