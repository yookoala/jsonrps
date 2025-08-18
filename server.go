package jsonrps

import (
	"bufio"
	"net"
	"net/http"
	"strings"
)

const (
	DefaultProtocolSignature = "JSONRPS/1.0"
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

	// Write protocol signature after receiving all headers but not blocking
	// session handler's take over
	go func() {
		c.Write([]byte(DefaultProtocolSignature + " 200 OK\r\n\r\n"))
	}()

	sess = s
	return
}
