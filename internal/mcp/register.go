package mcp

import (
	"encoding/json"
	"net"
	"os"
	"time"
)

// daemonSocket is cognitiveosd's Unix socket (cognitiveosd-api.md).
const daemonSocket = "/cognitiveos/run/daemon.sock"

// DaemonRegister best-effort announces the server to cognitiveosd. It never
// fails the process: when the daemon is not running (e.g. standalone or lab
// use), the server simply keeps serving over stdio.
func DaemonRegister(serverName string, tools []string) {
	conn, err := net.DialTimeout("unix", daemonSocket, 500*time.Millisecond)
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))

	msg := map[string]interface{}{
		"type": "mcp_register",
		"from": serverName,
		"payload": map[string]interface{}{
			"server": map[string]interface{}{
				"name":      serverName,
				"version":   Version,
				"transport": "stdio",
				"pid":       os.Getpid(),
			},
			"tools": tools,
		},
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return
	}
	raw = append(raw, '\n')
	if _, err := conn.Write(raw); err != nil {
		return
	}
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 4096)
	_, _ = conn.Read(buf)
}
