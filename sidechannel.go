// Side-channel REST server for the agent.
//
// iRUN starts this on the remote machine alongside the SSH server. The agent
// POSTs commands to it and gets stdout/stderr/exit_code back, bypassing SSH
// shell escaping entirely.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

const sideChannelPort = 2223

// sideChannelRequest and sideChannelResponse match the agent API.
type sideChannelRequest struct {
	Shell   string `json:"shell"`
	Command string `json:"command"`
}

type sideChannelResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// startSideChannel starts the REST server on 0.0.0.0:2223.
// It never returns; run it in a goroutine.
func startSideChannel() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	mux.HandleFunc("/exec", handleSideChannelExec)

	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", sideChannelPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] side channel: %v\n", err)
		return
	}
	fmt.Printf("  [+] side channel: http://0.0.0.0:%d\n", sideChannelPort)

	server := &http.Server{
		Addr:         ln.Addr().String(),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	_ = server.Serve(ln)
}

func handleSideChannelExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req sideChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Command == "" {
		http.Error(w, "command required", http.StatusBadRequest)
		return
	}

	shell := strings.ToLower(req.Shell)
	if shell == "" {
		shell = "cmd"
	}

	var cmd *exec.Cmd
	switch shell {
	case "powershell":
		cmd = exec.Command("powershell.exe", "-Command", req.Command)
	case "pwsh":
		cmd = exec.Command("pwsh.exe", "-Command", req.Command)
	default:
		cmd = exec.Command("cmd.exe", "/c", req.Command)
	}

	var outb, errb bytes.Buffer
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()

	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		code = 1
	}

	resp := sideChannelResponse{
		Stdout:   outb.String(),
		Stderr:   errb.String(),
		ExitCode: code,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
