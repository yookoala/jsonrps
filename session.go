package jsonrps

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Session is the raw I/O session between a server and a client
type Session struct {
	// ID is the internal identifier of a server-client session
	ID string

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

	// headerSent indicates if the headers have been sent
	headerSent bool
}

// WriteHeader sends the status code along with response header for the session.
func (sess *Session) WriteHeader(statusCode int) {
	fmt.Fprintf(sess.Conn, "%s %d %s\r\n", DefaultProtocolSignature, statusCode, http.StatusText(statusCode))

	// Also write headers to the connection
	for key, values := range sess.Headers {
		for _, value := range values {
			fmt.Fprintf(sess.Conn, "%s: %s\r\n", key, value)
		}
	}

	// Finish sending the header over
	fmt.Fprintf(sess.Conn, "\r\n")
	sess.headerSent = true
}

// Write writes the response body to the session.
func (sess *Session) Write(p []byte) (n int, err error) {
	if !sess.headerSent {
		// Finish sending the header over
		fmt.Fprintf(sess.Conn, "\r\n")
		sess.headerSent = true
	}
	return sess.Conn.Write(p)
}

// WriteRequest writes a JSON-RPC request to the session connection
// with an ending "\n"
func (sess *Session) WriteRequest(request *JSONRPCRequest) (err error) {
	var line []byte
	line, err = json.Marshal(request)
	if err != nil {
		return
	}
	_, err = sess.Write(append(line, '\n'))
	return
}

// ReadRequest reads a single line from the session connection,
// and they try to decoded it as JSON
func (sess *Session) ReadRequest() (request *JSONRPCRequest, err error) {
	var line string
	line, err = bufio.NewReader(sess.Conn).ReadString('\n')
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(line), &request)
	return
}

// WriteResponse writes a JSON-RPC response to the session connection
// with an ending "\n"
func (sess *Session) WriteResponse(response *JSONRPCResponse) (err error) {
	var line []byte
	line, err = json.Marshal(response)
	if err != nil {
		return
	}
	_, err = sess.Write(append(line, '\n'))
	return
}

// ReadResponse reads a JSON-RPC response from the session connection
// with an ending "\n"
func (sess *Session) ReadResponse() (response *JSONRPCResponse, err error) {
	var line string
	line, err = bufio.NewReader(sess.Conn).ReadString('\n')
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(line), &response)
	return
}

// Header returns the header map that will be sent by
// [Session.WriteHeader]. The [Header] map also is the mechanism with which
// [Handler] implementations can set HTTP trailers.
//
// Changing the header map after a call to [Session.WriteHeader] (or [Session.Write])
// has no effect unless the HTTP status code was of the
// 1xx class or the modified headers are trailers.
//
// There are two ways to set Trailers. The preferred way is to
// predeclare in the headers which trailers you will later
// send by setting the "Trailer" header to the names of the
// trailer keys which will come later. In this case, those
// keys of the Header map are treated as if they were
// trailers. See the example. The second way, for trailer
// keys not known to the [Handler] until after the first [ResponseWriter.Write],
// is to prefix the [Header] map keys with the [TrailerPrefix]
// constant value.
func (sess *Session) Header() http.Header {
	return sess.Headers
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

// CanHandleSession checks if any handler can handle the given session
func (r ServerSessionRouter) CanHandleSession(session *Session) bool {
	for _, handler := range r {
		if handler.CanHandleSession(session) {
			return true
		}
	}
	return false
}

// HandleSession routes the session to the appropriate handler
func (r ServerSessionRouter) HandleSession(session *Session) {
	for _, handler := range r {
		if handler.CanHandleSession(session) {
			handler.HandleSession(session)
			return
		}
	}
}
