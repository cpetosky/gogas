package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
)

// Represents a connection to a single IRC server
type IRCConn struct {
	net.Conn             // The underlying connection
	In, Out  chan string // Buffered input and output channels
	Closed   chan bool   // Written to once the connection is closed.
}

var ()

// Takes a connected IRCConn and populates its input and output channels
// until they are closed.
func listen(conn *IRCConn) {
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

// Connect to the provided server and return a listening IRCConn.
func connect(host, port, nick string) (*IRCConn, error) {
	conn, err := net.Dial("tcp", host+":"+port)
	if err != nil {
		return nil, err
	}

	in := make(chan string, 1000)
	out := make(chan string, 1000)
	irc := &IRCConn{conn, in, out, make(chan bool)}

	go listen(irc)
	irc.Out <- "NICK " + nick
	irc.Out <- "USER gogas 0 * :gogas v1"
	return irc, nil
}

// Spin up a single IRCConn to the server specified on the command line.
func main() {
	host := flag.String("host", "", "Host (like 'irc.freenode.org')")
	port := flag.String("port", "6667", "Port (like 6667)")
	nick := flag.String("nick", "", "IRC nick to use")
	flag.Parse()
	if *host == "" || *nick == "" {
		panic("Must supply host and nick at command line")
	}

	irc, err := connect(*host, *port, *nick)
	if err != nil {
		panic(err)
	}
	defer irc.Conn.Close()

	// Simple input handler -- just dump every line to the terminal.
	go func() {
		for str := range irc.In {
			fmt.Println(str)
		}
	}()

	<-irc.Closed
}
