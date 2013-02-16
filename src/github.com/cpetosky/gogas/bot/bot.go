package main

import (
	"flag"
	"fmt"
	"github.com/cpetosky/gogas/irc"
)

// Spin up a single Conn to the server specified on the command line.
func main() {
	host := flag.String("host", "", "Host (like 'irc.freenode.org')")
	port := flag.String("port", "6667", "Port (like 6667)")
	nick := flag.String("nick", "", "IRC nick to use")
	flag.Parse()
	if *host == "" || *nick == "" {
		panic("Must supply host and nick at command line")
	}

	conn, err := irc.Dial(*host, *port, *nick)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Simple input handler -- just dump every unhandled line to the terminal.
	go func() {
		for str := range conn.Unhandled {
			fmt.Println(str)
		}
	}()

	<-conn.Closed
}
