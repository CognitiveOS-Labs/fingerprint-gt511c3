package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/CognitiveOS-Labs/fingerprint-gt511c3/internal/device"
)

// roundTrip feeds requests to a Server over in-memory pipes and returns each
// parsed response line. It returns when in is closed.
func roundTrip(t *testing.T, s *Server, requests []string) []map[string]interface{} {
	t.Helper()
	var outBuf bytes.Buffer
	reqPipeR, reqPipeW := io.Pipe()
	defer reqPipeR.Close()
	done := make(chan error, 1)
	go func() {
		done <- s.Serve(reqPipeR, &outBuf)
	}()
	for _, r := range requests {
		reqPipeW.Write([]byte(r + "\n"))
	}
	reqPipeW.Close()
	if err := <-done; err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var out []map[string]interface{}
	sc := bufio.NewScanner(&outBuf)
	for sc.Scan() {
		var m map[string]interface{}
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("bad response line %q: %v", sc.Text(), err)
		}
		out = append(out, m)
	}
	return out
}

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return New(device.New(nil))
}

func TestListTools(t *testing.T) {
	s := newTestServer(t)
	outs := roundTrip(t, s, []string{`{"id":1,"method":"mcp_list_tools"}`})
	if len(outs) != 1 {
		t.Fatalf("expected 1 response, got %d", len(outs))
	}
	resp := outs[0]
	if resp["id"].(float64) != 1 {
		t.Fatalf("id = %v", resp["id"])
	}
	tools, ok := resp["tools"].([]interface{})
	if !ok {
		t.Fatalf("no tools array: %v", resp)
	}
	if len(tools) == 0 {
		t.Fatalf("no tools listed")
	}
	names := map[string]bool{}
	for _, ti := range tools {
		tm := ti.(map[string]interface{})
		name, _ := tm["name"].(string)
		names[name] = true
		if !strings.HasPrefix(name, ToolPrefix+".") {
			t.Fatalf("tool %q not prefixed", name)
		}
		if _, ok := tm["description"].(string); !ok {
			t.Fatalf("tool %q missing description", name)
		}
		if _, ok := tm["inputSchema"].(map[string]interface{}); !ok {
			t.Fatalf("tool %q missing inputSchema", name)
		}
	}
	for _, want := range []string{
		ToolPrefix + ".connect",
		ToolPrefix + ".enroll",
		ToolPrefix + ".identify",
		ToolPrefix + ".set_template",
		ToolPrefix + ".delete_all",
	} {
		if !names[want] {
			t.Fatalf("missing tool %s", want)
		}
	}
}

func TestUnknownMethod(t *testing.T) {
	s := newTestServer(t)
	outs := roundTrip(t, s, []string{`{"id":9,"method":"nope.not_real"}`})
	resp := outs[0]
	err, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if err["code"].(float64) != -32601 {
		t.Fatalf("code = %v", err["code"])
	}
}

func TestParseError(t *testing.T) {
	s := newTestServer(t)
	outs := roundTrip(t, s, []string{`{"id":1,`})
	resp := outs[0]
	err, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if err["code"].(float64) != -32700 {
		t.Fatalf("code = %v", err["code"])
	}
}

func TestHardwareErrorEnvelope(t *testing.T) {
	s := newTestServer(t)
	outs := roundTrip(t, s, []string{`{"id":3,"method":"` + ToolPrefix + `.get_enroll_count","params":{"arguments":{}}}`})
	resp := outs[0]
	if isErr, _ := resp["isError"].(bool); !isErr {
		t.Fatalf("expected isError, got %v", resp)
	}
	content := resp["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)
	if !strings.HasPrefix(text, "ERROR:E_HARDWARE:") {
		t.Fatalf("bad envelope: %q", text)
	}
}

func TestInvalidParamEnvelope(t *testing.T) {
	s := newTestServer(t)
	outs := roundTrip(t, s, []string{`{"id":4,"method":"` + ToolPrefix + `.verify","params":{"arguments":{"id":"notanumber"}}}`})
	resp := outs[0]
	if isErr, _ := resp["isError"].(bool); !isErr {
		t.Fatalf("expected isError, got %v", resp)
	}
	content := resp["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)
	if !strings.HasPrefix(text, "ERROR:E_INVALID_PARAM:") {
		t.Fatalf("bad envelope: %q", text)
	}
}

func TestIDOutOfRange(t *testing.T) {
	s := newTestServer(t)
	outs := roundTrip(t, s, []string{`{"id":5,"method":"` + ToolPrefix + `.check_enrolled","params":{"arguments":{"id":300}}}`})
	text := outs[0]["content"].([]interface{})[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(text, "0-199") {
		t.Fatalf("expected range error, got %q", text)
	}
}

func TestStatusNotConnected(t *testing.T) {
	s := newTestServer(t)
	outs := roundTrip(t, s, []string{`{"id":6,"method":"` + ToolPrefix + `.status","params":{"arguments":{}}}`})
	text := outs[0]["content"].([]interface{})[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(text, `"connected":false`) {
		t.Fatalf("expected connected:false, got %q", text)
	}
}

func TestShutdown(t *testing.T) {
	s := newTestServer(t)
	outs := roundTrip(t, s, []string{
		`{"id":1,"method":"mcp_list_tools"}`,
		`{"id":2,"method":"mcp_shutdown"}`,
	})
	if len(outs) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(outs))
	}
	if outs[1]["id"].(float64) != 2 {
		t.Fatalf("shutdown id = %v", outs[1]["id"])
	}
}

func TestHealthcheck(t *testing.T) {
	s := newTestServer(t)
	outs := roundTrip(t, s, []string{`{"type":"healthcheck"}`})
	resp := outs[0]
	if resp["type"] != "healthcheck_ok" {
		t.Fatalf("type = %v", resp["type"])
	}
	if healthy, _ := resp["tools_healthy"].(bool); !healthy {
		t.Fatalf("tools_healthy = %v", resp["tools_healthy"])
	}
	if _, ok := resp["uptime_seconds"].(float64); !ok {
		t.Fatalf("missing uptime_seconds")
	}
}
