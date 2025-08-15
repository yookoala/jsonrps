package jsonrps

import (
	"bufio"
	"net"
	"net/http"
	"strings"
)

const (
	DefaultProtocolSignature = "JSON-RPC/1.0"
)

// HandleServerConn provides default connection handling logic of server.
func HandleServerConn(c net.Conn, r ServerSessionRouter) {
	sess := &Session{
		ProtocolSignature: DefaultProtocolSignature,
		Conn:              c,
		Headers:           make(http.Header),
	}

	// Read each line as if it is HTTP header into sess.Headers
	// and stop when reaching "\n\n"
	for {
		line, err := bufio.NewReader(c).ReadString('\n')
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
			sess.Headers.Add(parts[0], strings.TrimSpace(parts[1]))
		}
	}

	// Write protocol signature after receiving all headers but not blocking
	// session handler's take over
	go func() {
		c.Write([]byte(DefaultProtocolSignature + " 200 OK\r\n\r\n"))
	}()

	r.HandleSession(sess)
}
