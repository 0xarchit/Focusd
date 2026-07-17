package coreapi

import (
	"bufio"
	"net"
	"strings"
	"time"
)

const IPCAddress = "127.0.0.1:48321"

func SendIPCCmd(cmd string) bool {
	conn, err := net.DialTimeout("tcp", IPCAddress, 1*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(1 * time.Second)); err != nil {
		return false
	}

	_, err = conn.Write([]byte(cmd + "\n"))
	if err != nil {
		return false
	}

	response, err := bufio.NewReader(conn).ReadString('\n')
	return err == nil && strings.TrimSpace(response) == "ok"
}
