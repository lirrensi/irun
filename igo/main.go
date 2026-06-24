// igo: human-only interactive SSH login for iRUN servers.
//
//	igo
//
// Scans the LAN for iRUN servers on port 2222. If one is found, connects
// immediately. If several are found, prints a numbered list and asks the user
// to pick one. Then opens an interactive PTY shell on the remote machine.
//
// Also starts a localhost REST side-channel so the agent can run commands on
// this machine without dealing with Windows shell escaping. The human never
// interacts with it.
//
// Does absolutely nothing else from the human's point of view.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	remotePort = 2222
	localPort  = 4222
	localMax   = 4299
)

func main() {
	startSideChannel()

	fmt.Println("[*] scanning for iRUN servers ...")
	servers := scan()

	switch len(servers) {
	case 0:
		fmt.Fprintln(os.Stderr, "[!] no iRUN servers found")
		os.Exit(1)
	case 1:
		fmt.Printf("[+] 1 server found: %s\n", servers[0])
		connect(servers[0])
	default:
		fmt.Printf("[+] %d servers found:\n", len(servers))
		for i, ip := range servers {
			fmt.Printf("    %d) %s\n", i+1, ip)
		}
		ip := pick(servers)
		connect(ip)
	}
}

// ---- side channel --------------------------------------------------------

type execRequest struct {
	Shell   string `json:"shell"`
	Command string `json:"command"`
}

type execResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

func startSideChannel() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	mux.HandleFunc("/exec", handleExec)

	ln := bindLocalPort()
	port := ln.Addr().(*net.TCPAddr).Port
	writePortFile(port)
	fmt.Printf("[+] side channel: http://127.0.0.1:%d\n", port)

	go func() {
		_ = http.Serve(ln, mux)
	}()
}

func bindLocalPort() net.Listener {
	for p := localPort; p <= localMax; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return ln
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] side channel: %v\n", err)
		os.Exit(1)
	}
	return ln
}

func writePortFile(port int) {
	dir := cacheDir()
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "igo.port")
	_ = os.WriteFile(path, []byte(strconv.Itoa(port)), 0644)
}

func cacheDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".irun")
	}
	return os.TempDir()
}

func handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req execRequest
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

	resp := execResponse{
		Stdout:   outb.String(),
		Stderr:   errb.String(),
		ExitCode: code,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// ---- scanner --------------------------------------------------------------

func scan() []string {
	prefixes := autoDetectPrefixes()
	if len(prefixes) == 0 {
		fmt.Fprintln(os.Stderr, "[!] no subnet detected")
		os.Exit(2)
	}

	var (
		mu    sync.Mutex
		found []string
		wg    sync.WaitGroup
		sem   = make(chan struct{}, 64)
	)
	for _, prefix := range prefixes {
		for i := 1; i < 255; i++ {
			wg.Add(1)
			sem <- struct{}{}
			go func(pref string, n int) {
				defer wg.Done()
				defer func() { <-sem }()
				ip := fmt.Sprintf("%s.%d", pref, n)
				if isIRUN(ip) {
					mu.Lock()
					found = append(found, ip)
					mu.Unlock()
				}
			}(prefix, i)
		}
	}
	wg.Wait()
	sort.Strings(found)
	return found
}

func isIRUN(ip string) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, remotePort), 500*time.Millisecond)
	if err != nil {
		return false
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}
	return strings.Contains(string(buf[:n]), "iRUN")
}

func autoDetectPrefixes() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		name := strings.ToLower(iface.Name)
		if strings.Contains(name, "vmware") || strings.Contains(name, "hyper-v") ||
			strings.Contains(name, "vethernet") || strings.Contains(name, "virtual") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil {
				continue
			}
			ip := ip4.String()
			if strings.HasPrefix(ip, "169.254.") {
				continue
			}
			parts := strings.Split(ip, ".")
			if len(parts) == 4 {
				prefix := parts[0] + "." + parts[1] + "." + parts[2]
				if !seen[prefix] {
					seen[prefix] = true
					out = append(out, prefix)
				}
			}
		}
	}
	return out
}

// ---- connection -----------------------------------------------------------

func pick(servers []string) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Pick one (1-%d): ", len(servers))
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintln(os.Stderr, "[!] read error")
			os.Exit(1)
		}
		line = strings.TrimSpace(line)
		n, err := strconv.Atoi(line)
		if err == nil && n >= 1 && n <= len(servers) {
			return servers[n-1]
		}
		fmt.Println("[!] invalid choice")
	}
}

func connect(ip string) {
	fmt.Printf("[+] connecting to %s ...\n", ip)

	user := os.Getenv("USERNAME")
	if user == "" {
		user = "user"
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, remotePort), cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[!] session: %v\n", err)
		os.Exit(1)
	}
	defer sess.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm", 120, 40, modes); err != nil {
		fmt.Fprintf(os.Stderr, "[!] pty: %v\n", err)
		os.Exit(1)
	}

	sess.Stdin = os.Stdin
	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr

	if err := sess.Shell(); err != nil {
		fmt.Fprintf(os.Stderr, "[!] shell: %v\n", err)
		os.Exit(1)
	}
	sess.Wait()
}
