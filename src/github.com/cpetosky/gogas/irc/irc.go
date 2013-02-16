package irc

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

type Message struct {
	Prefix  string   // Origin of the message, can be nil
	Command string   // IRC command or 3-digit reply code
	Params  []string // Params for the command, maximum of 15
}

func parseMessage(msg string) Message {
	// Strip off \r\n
	msg = msg[:len(msg)-2]

	var prefix string

	tokens := strings.Split(msg, " ")

	// Check for optional prefix and store it without leading colon
	if tokens[0][0] == ':' {
		prefix = tokens[0][1:]
		tokens = tokens[1:]
	}
	command := tokens[0]
	tokens = tokens[1:]

	params := make([]string, 0, 15)
	for len(tokens) > 0 && len(params) < 14 {
		// A token starting with ":" is the final token, and can have spaces.
		if tokens[0][0] == ':' {
			tokens[0] = tokens[0][1:]
			break
		}
		params = append(params, tokens[0])
		tokens = tokens[1:]
	}

	// Any leftover tokens are really one final token with spaces allowed.
	if len(tokens) > 0 {
		params = append(params, strings.Join(tokens, " "))
	}

	return Message{prefix, command, params}
}

func (message Message) String() string {
	return fmt.Sprintf(":%s %s %s", message.Prefix,
		message.Command, strings.Join(message.Params, " "))
}

// Represents a connection to a single IRC server
type Conn struct {
	net.Conn              // The underlying TCP connection
	Out      chan string  // Buffered channel for outgoing messages
	In       chan Message // Contains all parsed incoming messages
	Closed   chan bool    // Written to once the connection is closed.
}

// Dial connects to the provided server and returns a listening Conn.
func Dial(host, port, nick string) (*Conn, error) {
	tcpConn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return nil, err
	}

	in := make(chan string, 1000)
	out := make(chan Message, 1000)
	conn := &Conn{tcpConn, in, out, make(chan bool)}

	conn.listen()
	conn.Out <- "NICK " + nick
	conn.Out <- "USER " + nick + " 0 * :" + nick
	return conn, nil
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
			str, err := reader.ReadString(byte('\n'))
			if err != nil {
				fmt.Fprintf(os.Stderr, "irc: read error: %s\n", err)
				conn.Closed <- true
				break
			}

			message := parseMessage(str)

			// IRC PING messages must be responded to with PONG + the ID provided.
			// Handle this here so command processors don't have to think about it.
			if message.Command == "PING" {
				conn.Out <- "PONG " + message.Params[0]
			} else {
				conn.In <- message
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
