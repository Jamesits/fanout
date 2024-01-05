package main

import (
	"fmt"
	"io"
	"log"
	"net"
)

var cnt int // for round-robin counting

// https://github.com/m13253/popub/blob/a96c877dbf168309d73525a7987dd0d26e5c03cc/popub-relay/main.go#L194-L198
func copyTCPConn(dst, src *net.TCPConn) {
	var err error
	_, err = io.Copy(dst, src)
	if err != nil {
		log.Printf("WRITE error: %v\n", err)
	}
	_ = src.CloseRead()
	_ = dst.CloseWrite()
}

func handleTCPConn(listener *net.TCPListener) error {
	conn, err := listener.AcceptTCP()
	if err != nil {
		log.Printf("ACCEPT error: %v\n", err)
		return err
	}

	mu.RLock()
	defer mu.RUnlock()

	// get a possible endpoint (round-robin)
	cnt = (cnt + 1) % len(sg)
	dst, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", sg[cnt].Address, sg[cnt].Port))

	// connect
	proxyConn, err := net.DialTCP("tcp", nil, dst)
	if err != nil {
		log.Printf("DIAL error: %v\n", err)
		return err
	}
	log.Printf("CONN accept: %s --> %s - %s --> %s(%s)\n", conn.RemoteAddr(), conn.LocalAddr(), proxyConn.LocalAddr(), dst.String(), proxyConn.RemoteAddr())

	go copyTCPConn(proxyConn, conn)
	go copyTCPConn(conn, proxyConn)
	return nil
}
