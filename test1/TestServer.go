package main

import (
	"fmt"
	"net"
)

func main() {
	l, err := net.Listen("tcp", ":8899")
	if err != nil {
		panic(err)
	}
	fmt.Println("listen to 8899")
	for {
		conn, err := l.Accept()
		if err != nil {
			panic(err)
		} else {
			go handleConn(conn)
		}
	}
}
func handleConn(conn net.Conn) {
	defer conn.Close()
	var buf [1024]byte
	for {
		n, err := conn.Read(buf[:])
		if err != nil {
			break
		} else {
			fmt.Printf("recv: %s ", string(buf[0:n]))
		}
	}
}
