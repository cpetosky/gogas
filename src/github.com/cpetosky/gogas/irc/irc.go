package irc

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

const (
	ReplyWelcome = "001"
	CmdPing = "PING"
	CmdPong = "PONG"
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

// Represents a connection to a single IRC server.
type Conn struct {
	net.Conn                          // The underlying TCP connection.
	Out       chan string             // Buffered channel for outgoing messages.
	In        map[string]chan Message // Hook to add  handlers based on command.
	Unhandled chan Message            // Contains all unhandled incoming messages.
	Closed    chan bool               // Written to once the connection is closed.
}

// Dial connects to the provided server and returns a listening Conn.
func Dial(host, port, nick string) (*Conn, error) {
	tcpConn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return nil, err
	}

	out := make(chan string, 1000)
	in := make(map[string]chan Message)
	unhandled := make(chan Message, 1000)
	conn := &Conn{tcpConn, out, in, unhandled, make(chan bool)}

	// IRC PING messages must be responded to with PONG + the ID provided.
	// Handle this here so command processors don't have to think about it.
	conn.In[CmdPing] = make(chan Message, 1)
	go func() {
		for message := range conn.In[CmdPing] {
			conn.Out <- CmdPong + " " + message.Params[0]
		}
	}()

	conn.In[ReplyWelcome] = make(chan Message, 1)
	conn.listen()
	conn.Out <- "NICK " + nick
	conn.Out <- "USER " + nick + " 0 * :" + nick

	// Don't return until we successfully register with the server.
	<-conn.In[ReplyWelcome]
	delete(conn.In, ReplyWelcome)
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
			if conn.In[message.Command] != nil {
				conn.In[message.Command] <- message
			} else {
				conn.Unhandled <- message
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
