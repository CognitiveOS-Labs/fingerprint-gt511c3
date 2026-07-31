// gt511c3-mcp is the CognitiveOS MCP server for the ADH Tech GT-511C3
// fingerprint scanner. It serves newline-delimited JSON over stdio
// (mcp-conventions.md) exposing cognitiveos.labs.fingerprint-gt511c3.* tools.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/CognitiveOS-Labs/fingerprint-gt511c3/internal/device"
	"github.com/CognitiveOS-Labs/fingerprint-gt511c3/internal/mcp"
)

func main() {
	var (
		showVersion = flag.Bool("version", false, "print version and exit")
		port        = flag.String("port", "", "serial port to auto-connect on startup")
		baud        = flag.Int("baud", 0, "baud rate for auto-connect (0 = scan)")
		stdio       = flag.Bool("stdio", true, "serve over stdio (default true)")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("gt511c3-mcp %s\n", mcp.Version)
		return
	}
	if !*stdio {
		fmt.Fprintln(os.Stderr, "gt511c3-mcp: only stdio transport is supported")
		os.Exit(1)
	}

	dev := device.New(nil)
	if *port != "" {
		if err := dev.Connect(*port, *baud); err != nil {
			fmt.Fprintf(os.Stderr, "gt511c3-mcp: connect %s: %v\n", *port, err)
			os.Exit(1)
		}
	}

	server := mcp.New(dev)
	server.SetRegister(func() {
		mcp.DaemonRegister("fingerprint-gt511c3-mcp", server.ToolNames())
	})

	if err := server.Serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "gt511c3-mcp: %v\n", err)
		os.Exit(1)
	}
}
