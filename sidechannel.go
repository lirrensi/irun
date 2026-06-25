// Side-channel REST server for the agent.
//
// iRUN starts this on the remote machine alongside the SSH server. The agent
// POSTs commands to it and gets stdout/stderr/exit_code back, bypassing SSH
// shell escaping entirely.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	mux.HandleFunc("/push", handleSideChannelPush)
	mux.HandleFunc("/pull", handleSideChannelPull)

	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", sideChannelPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] side channel: %v\n", err)
		return
	}
	fmt.Printf("  [+] side channel: http://0.0.0.0:%d\n", sideChannelPort)

	server := &http.Server{
		Addr:    ln.Addr().String(),
		Handler: mux,
		// No read or write timeout — file transfers and long
		// commands may take minutes.
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
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	switch shell {
	case "powershell":
		cmd = exec.CommandContext(ctx, "powershell.exe", "-Command", req.Command)
	case "pwsh":
		cmd = exec.CommandContext(ctx, "pwsh.exe", "-Command", req.Command)
	default:
		cmd = exec.CommandContext(ctx, "cmd.exe", "/c", req.Command)
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

func handleSideChannelPush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "PUT only", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	f, err := os.Create(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, r.Body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func handleSideChannelPull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	io.Copy(w, f)
}
