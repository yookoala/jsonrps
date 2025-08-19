package jsonrps

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
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
	go func() {
		// Write HTTP header to connection
		for key, values := range h {
			for _, value := range values {
				c.Write([]byte(key + ": " + value + "\r\n"))
			}
		}
		c.Write([]byte("\r\n"))
	}()

	// Read response for protocol signature
	// Check against the DefaultProtocolSignature
	if sess.ProtocolSignature != DefaultProtocolSignature {
		err = errors.New("invalid protocol signature: " + sess.ProtocolSignature)
		c.Close()
		return
	}

	// Read header into sess.Headers
	for {
		// Read HTTP headers from connection
		line, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			break
		}
		if line == "\r\n" {
			break
		}
		// Parse header line
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			sess.LocalHeaders.Add(parts[0], strings.TrimSpace(parts[1]))
		}
	}

	// Return the header and the session for further processing
	return
}
