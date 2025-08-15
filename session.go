package jsonrps

import (
	"context"
	"io"
	"net/http"
)

// Session is the raw I/O session between a server and a client
type Session struct {
	// ProtocolSignature is the signature of the protocol being used
	// by the server of the protocol type and version
	ProtocolSignature string

	// Headers is the HTTP headers associated with the session
	Headers http.Header

	// Context is the context associated with the session for
	// cancellation
	Context context.Context

	// Conn is the session connection between a server and a client
	Conn io.ReadWriteCloser
}

// SessionHandler is a generic interface for handling sessions
// on either the client or the server side.
type SessionHandler interface {
	HandleSession(session *Session)
}

// ServerSessionHandler is an interface for handling server sessions
type ServerSessionHandler interface {
	SessionHandler

	// CanHandleSession checks if the handler can handle the given session
	CanHandleSession(session *Session) bool
}

// ServerSessionRouter is an interface for routing server sessions
// to proper ServerSessionHandler
type ServerSessionRouter []ServerSessionHandler

// HandleSession routes the session to the appropriate handler
func (r ServerSessionRouter) HandleSession(session *Session) {
	for _, handler := range r {
		if handler.CanHandleSession(session) {
			handler.HandleSession(session)
			return
		}
	}
}
