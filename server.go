package jsonrps

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const (
	// Default protocol signature
	DefaultProtocolSignature = "JSONRPS/1.0"

	// Default MIME type of the RPC + PubSub content
	DefaultMimeType = "application/json+rps"
)

// InitializeServerSession provides default connection handling logic of server.
func InitializeServerSession(logger *slog.Logger, c net.Conn) (sess *Session, err error) {
	sid := uuid.New().String()
	s := &Session{
		ID:                sid,
		ProtocolSignature: DefaultProtocolSignature,
		Conn:              c,
		Headers:           make(http.Header),
		Logger:            logger.With("session_id", sid),
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

	Close() error
}

// NewServer creates a new instance of default implementation
// of ServerSessionHandler.
func NewServer() Server {
	return &defaultServer{
		mimeType:    DefaultMimeType,
		methods:     make(map[string]Method),
		methodsLock: sync.Mutex{},
		sessions:    make(map[*Session]struct{}),
		closed:      make(chan struct{}),
	}
}

// defaultServer is the default implementation of ServerSessionHandler.
type defaultServer struct {
	mimeType string

	methods     map[string]Method
	methodsLock sync.Mutex

	// sessions store all sessions that the server is connected to
	sessions map[*Session]struct{}

	// closed is a channel for closing the server
	closed chan struct{}
}

// CanHandleSession checks if the server can handle the given session.
func (h *defaultServer) CanHandleSession(session *Session) bool {
	return session.Headers.Get("Accept") == h.mimeType
}

// HandleSession handles the incoming session.
// This method runs for the duration of the session, managing the session's
// request/response lifecycle until the session connection is closed or an error occurs.
func (h *defaultServer) HandleSession(sess *Session) {
	reqQueue := make(chan *JSONRPCRequest, 200)
	respQueue := make(chan *JSONRPCResponse, 200)
	sessionDone := make(chan struct{})
	var sessionDoneOnce sync.Once
	var methodsWG sync.WaitGroup

	// Helper function to safely close sessionDone only once
	cleanUp := func() {
		sessionDoneOnce.Do(func() {
			close(sessionDone)
		})
	}

	// Reader goroutine
	go func() {
		defer close(reqQueue)
		// Loop to read all request from session into readQueue
		for {
			req, err := sess.ReadRequest()
			if err != nil {
				// Note: supposed the ReadRequest call would result in some
				// read error if the connection is closed. That will terminate
				// this goroutine here.
				sess.Logger.Error("failed to read request", "error", err)
				cleanUp() // Signal that the session is done
				return
			}
			reqQueue <- req
		}
	}()

	// Request handler goroutine
	go func() {
		defer func() {
			// Wait for all method goroutines to complete before closing respQueue
			methodsWG.Wait()
			close(respQueue)
		}()
		// Loop to read everything from requestQueue and handle requests
		for {
			select {
			case <-sessionDone:
				// Session is done, stop processing
				return
			case <-h.closed:
				// Server is shutting down, stop processing
				return
			case req, ok := <-reqQueue:
				if !ok {
					// Request queue closed, stop processing
					return
				}
				if m, ok := h.methods[req.Method]; ok {
					methodsWG.Add(1)
					go func(request *JSONRPCRequest) {
						defer methodsWG.Done()
						resp, err := m(context.Background(), request)
						if err != nil {
							sess.Logger.Error("failed to handle request", "error", err, "request", request)
							return
						}
						// Try to send response, but don't panic if channel is closed
						select {
						case respQueue <- resp:
							// Successfully sent response
						case <-sessionDone:
							// Session closed while trying to send response
						case <-h.closed:
							// Server closed while trying to send response
						}
					}(req)
				}
			}
		}
	}()

	// Response writer goroutine
	go func() {
		for {
			select {
			case <-sessionDone:
				// Session is done, stop writing responses
				return
			case <-h.closed:
				// Server is shutting down, stop writing responses
				return
			case resp, ok := <-respQueue:
				if !ok {
					// Response queue closed, stop writing
					return
				}
				err := sess.WriteResponse(resp)
				if err != nil {
					sess.Logger.Error("failed to write response", "error", err)
					cleanUp() // Signal session done on write error
					return
				}
			}
		}
	}()

	// Wait until the session is closed
	<-sessionDone
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

func (h *defaultServer) Close() error {
	close(h.closed)
	return nil
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
