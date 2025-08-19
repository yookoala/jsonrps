package jsonrps

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

func DefaultClientHeader() http.Header {
	h := make(http.Header)
	h.Add("Accept", DefaultMimeType)
	return h
}

func InitializeClientSession(c net.Conn, h http.Header, l *slog.Logger) (sess *Session, err error) {
	sess = &Session{
		ProtocolSignature: DefaultProtocolSignature,
		Conn:              c,
		LocalHeaders:      h,
		RemoteHeaders:     make(http.Header),
		Logger:            l,
	}

	// Write the HTTP header without blocking reads
	sess.WriteClientHeader()

	// Read response for protocol signature
	// Check against the DefaultProtocolSignature
	if sess.ProtocolSignature != DefaultProtocolSignature {
		sess.Logger.Error("Invalid protocol signature", "signature", sess.ProtocolSignature)
		err = errors.New("invalid protocol signature: " + sess.ProtocolSignature)
		sess.Close()
		return
	}

	sess.WriteRequest(&JSONRPCRequest{
		Version: "2.0",
		ID:      uuid.New().String(),
		Method:  "ping",
		Params:  nil,
	})

	// Read header into sess.Headers
	for {
		// Read HTTP headers from connection
		sess.Logger.Debug("read string start")
		line, err := bufio.NewReader(c).ReadString('\n')
		sess.Logger.Debug("read string finished")
		line = strings.Trim(line, "\r\n ")
		sess.Logger.Debug("Reading header line", "line", line, "err", err)
		if err != nil {
			break
		}
		if line == "" {
			sess.Logger.Debug("Reach server's header terminator")
			break
		}
		// Parse header line
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			sess.LocalHeaders.Add(parts[0], strings.TrimSpace(parts[1]))
		}
	}

	sess.Logger.Debug("Finished reading header", "headers", sess.LocalHeaders)

	// Return the header and the session for further processing
	return
}
