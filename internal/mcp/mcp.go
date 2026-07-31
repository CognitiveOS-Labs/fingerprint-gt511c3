// Package mcp implements the CognitiveOS MCP server for the GT-511C3
// fingerprint scanner. It speaks newline-delimited JSON over stdio
// (mcp-conventions.md): mcp_list_tools for discovery, tool-name methods for
// invocation, healthcheck, and mcp_shutdown for lifecycle.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/CognitiveOS-Labs/fingerprint-gt511c3/internal/device"
	"github.com/CognitiveOS-Labs/fingerprint-gt511c3/internal/protocol"
)

// Version of the MCP server, matched with the package manifest.
var Version = "0.1.1"

// ToolPrefix is the reverse-domain tool prefix for this custom patch:
// <publisher>.<patch-name>.<action> (mcp-conventions.md rule 4).
const ToolPrefix = "com.cognitiveos.labs.fingerprint-gt511c3"

// ToolResult is a single content block returned from a tool invocation.
type ToolResult struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// toolHandler executes a tool with parsed arguments and returns a JSON value
// to be rendered as the text result, or an error to be mapped to an ERROR
// envelope.
type toolHandler func(args map[string]interface{}) (interface{}, error)

// tool is a registered MCP tool definition and its handler.
type tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
	handler     toolHandler
}

// rpcError is a JSON-RPC level error (unknown method, bad request).
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Server serves MCP requests over an io.Reader/io.Writer pair.
type Server struct {
	dev      *device.Device
	tools    []tool
	toolMap  map[string]tool
	started  time.Time
	register func()
}

// New creates an MCP server bound to the given device.
func New(dev *device.Device) *Server {
	s := &Server{
		dev:     dev,
		tools:   buildTools(dev),
		started: time.Now(),
	}
	s.toolMap = make(map[string]tool, len(s.tools))
	for _, t := range s.tools {
		s.toolMap[t.Name] = t
	}
	return s
}

// ToolNames returns the fully-qualified names of all served tools.
func (s *Server) ToolNames() []string {
	names := make([]string, 0, len(s.tools))
	for _, t := range s.tools {
		names = append(names, t.Name)
	}
	return names
}

// SetRegister installs the best-effort daemon registration callback.
func (s *Server) SetRegister(fn func()) {
	s.register = fn
}

// Serve reads requests from in and writes responses to out until EOF. A
// mcp_shutdown request ends the loop.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 256*1024), 4*1024*1024)
	writer := bufio.NewWriter(out)

	if s.register != nil {
		go s.register()
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		resp, done := s.handle(line)
		if resp == nil {
			continue
		}
		raw, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		if _, err := writer.Write(raw); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		if done {
			return nil
		}
	}
	return scanner.Err()
}

// handle processes one request line. The returned response may be nil (e.g.
// healthcheck messages which have their own envelope).
func (s *Server) handle(line []byte) (interface{}, bool) {
	var req struct {
		ID     interface{}            `json:"id"`
		Type   string                 `json:"type"`
		Method string                 `json:"method"`
		Params map[string]interface{} `json:"params"`
	}
	if err := json.Unmarshal(line, &req); err != nil {
		return response(req.ID, &rpcError{Code: -32700, Message: "parse error: " + err.Error()}), false
	}

	method := req.Method
	if method == "" {
		method = req.Type
	}

	switch method {
	case "mcp_list_tools":
		tools := make([]tool, len(s.tools))
		copy(tools, s.tools)
		return struct {
			ID    interface{} `json:"id"`
			Tools []tool      `json:"tools"`
		}{ID: req.ID, Tools: tools}, false
	case "mcp_shutdown":
		return struct {
			ID      interface{}  `json:"id"`
			Content []ToolResult `json:"content"`
		}{ID: req.ID, Content: []ToolResult{{Type: "text", Text: "shutting down"}}}, true
	case "healthcheck", "mcp_healthcheck":
		return struct {
			Type          string `json:"type"`
			UptimeSeconds int64  `json:"uptime_seconds"`
			ToolsHealthy  bool   `json:"tools_healthy"`
		}{Type: "healthcheck_ok", UptimeSeconds: int64(time.Since(s.started).Seconds()), ToolsHealthy: true}, false
	}

	t, ok := s.toolMap[method]
	if !ok {
		return response(req.ID, &rpcError{Code: -32601, Message: "Unknown method: " + method}), false
	}

	args := map[string]interface{}{}
	if params, ok := req.Params["arguments"].(map[string]interface{}); ok {
		args = params
	}

	result, err := t.handler(args)
	if err != nil {
		return struct {
			ID      interface{}  `json:"id"`
			IsError bool         `json:"isError"`
			Content []ToolResult `json:"content"`
		}{
			ID:      req.ID,
			IsError: true,
			Content: []ToolResult{{Type: "text", Text: "ERROR:" + errorCode(err) + ":" + err.Error()}},
		}, false
	}
	text, err := json.Marshal(result)
	if err != nil {
		text = []byte(fmt.Sprintf("%v", result))
	}
	return struct {
		ID      interface{}  `json:"id"`
		Content []ToolResult `json:"content"`
	}{
		ID:      req.ID,
		Content: []ToolResult{{Type: "text", Text: string(text)}},
	}, false
}

func response(id interface{}, rpcErr *rpcError) interface{} {
	return struct {
		ID    interface{} `json:"id"`
		Error *rpcError   `json:"error"`
	}{ID: id, Error: rpcErr}
}

// errorCode maps a tool error to the standard CognitiveOS error code.
func errorCode(err error) string {
	switch e := err.(type) {
	case *protocol.NackError:
		return "E_HARDWARE"
	case *device.TimeoutError:
		return "E_TIMEOUT"
	case *argError:
		return "E_INVALID_PARAM"
	default:
		if e == device.ErrNotConnected {
			return "E_HARDWARE"
		}
		if os.IsNotExist(err) {
			return "E_NOT_FOUND"
		}
		return "E_INTERNAL"
	}
}
