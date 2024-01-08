package main

import (
	"fmt"
	"io"
	"net"
)

// https://github.com/m13253/popub/blob/a96c877dbf168309d73525a7987dd0d26e5c03cc/popub-relay/main.go#L194-L198
func copyTCPConn(dst, src *net.TCPConn) {
	defer connWG.Done()

	var err error
	_, err = io.Copy(dst, src)
	if err != nil {
		errorLogger.Error("TCP write failed", "error", err)
	}
	_ = src.CloseRead()
	_ = dst.CloseWrite()
}

func handleTCPConn(listener *net.TCPListener) {
	var cnt int // for round-robin counting

	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			errorLogger.Error("accept connection failed", "local_address", conn.LocalAddr(), "remote_address", conn.RemoteAddr(), "error", err)
			continue
		}
        // errorLogger.Debug("new connection", "incoming_remote_address", conn.RemoteAddr(), "incoming_local_address", conn.LocalAddr())

		// get a possible endpoint
		mu.RLock()
		if len(sg) == 0 {
			errorLogger.Warn("no upstream found")
			_ = conn.Close()
            mu.RUnlock()
			continue
		}
		cnt = (cnt + 1) % len(sg) // round-robin
		upstreamAddr := fmt.Sprintf("%s:%d", sg[cnt].Address, sg[cnt].Port)
		mu.RUnlock()
		dst, err := net.ResolveTCPAddr("tcp", upstreamAddr)
		if err != nil {
			errorLogger.Error("resolve upstream address failed", "address", upstreamAddr)
			_ = conn.Close()
			continue
		}

		// connect
		proxyConn, err := net.DialTCP("tcp", nil, dst)
		if err != nil {
			errorLogger.Error("create proxy connection failed", "error", err, "address", dst)
			_ = conn.Close()
			continue
		}

		// forward
		accessLogger.Info("new connection", "incoming_remote_address", conn.RemoteAddr(), "incoming_local_address", conn.LocalAddr(), "upstream_local_address", proxyConn.LocalAddr(), "upstream_remote_raw_address", dst.String(), "upstream_remote_address", proxyConn.RemoteAddr())
		connWG.Add(2)
		go copyTCPConn(proxyConn, conn)
		go copyTCPConn(conn, proxyConn)
	}
}
