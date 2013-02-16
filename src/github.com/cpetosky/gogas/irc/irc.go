package irc

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

// Represents a connection to a single IRC server
type Conn struct {
	net.Conn             // The underlying TCP connection
	In, Out  chan string // Buffered input and output channels
	Closed   chan bool   // Written to once the connection is closed.
}

// Dial connects to the provided server and returns a listening Conn.
func Dial(host, port, nick string) (*Conn, error) {
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return nil, err
	}

	in := make(chan string, 1000)
	out := make(chan string, 1000)
	irc := &Conn{conn, in, out, make(chan bool)}

	irc.listen()
	irc.Out <- "NICK " + nick
	irc.Out <- "USER " + nick + " 0 * :" + nick
	return irc, nil
}

// Listen instructs the Conn to start sending and receiving data over its
// In and Out channels. It will continue servicing these channels until
// the backing TCP connection is closed.
func (conn *Conn) listen() {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Process all messages from the server into the In channel.
	go func() {
		for {
			if str, err := reader.ReadString(byte('\n')); err != nil {
				fmt.Fprintf(os.Stderr, "irc: read error: %s\n", err)
				conn.Closed <- true
				break
			} else {
				// Strip off \r\n
				msg := str[:len(str)-2]

				// IRC PING messages must be responded to with PONG + the ID provided.
				// Handle this here so command processors don't have to think about it.
				if msg[:4] == "PING" {
					conn.Out <- "PONG" + msg[4:]
				} else {
					conn.In <- msg
				}
			}
		}
	}()

	// Send all output in the Out channel to the server.
	go func() {
		for str := range conn.Out {
			if _, err := writer.WriteString(str + "\n\r"); err != nil {
				fmt.Fprintf(os.Stderr, "irc: write error: %s\n", err)
				conn.Closed <- true
				break
			} else {
				writer.Flush()
				fmt.Println("wrote:", str)
			}
		}
	}()
}
