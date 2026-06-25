// igo: human-only interactive SSH login for iRUN servers.
//
//	igo
//	igo 192.168.66.78
//
// Scans the LAN for iRUN servers on port 2222. If one is found, connects
// immediately. If several are found, prints a numbered list and asks the user
// to pick one. Then opens an interactive PTY shell on the remote machine.
//
// Does absolutely nothing else.
package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

const remotePort = 2222
const sideChannelPort = 2223

// fatal prints an error and pauses when running in a terminal so the window
// does not close before the user can read the message.
func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "\nPress Enter to exit...")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(1)
}

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "push":
			if len(os.Args) != 4 {
				fatal("usage: igo push <local_path> <remote_path>\n")
			}
			pushCmd(os.Args[2], os.Args[3])
			return
		case "pull":
			if len(os.Args) != 4 {
				fatal("usage: igo pull <remote_path> <local_path>\n")
			}
			pullCmd(os.Args[2], os.Args[3])
			return
		}
	}

	if len(os.Args) > 1 {
		connect(os.Args[1])
		return
	}

	fmt.Println("[*] scanning for iRUN servers ...")
	servers := scan()

	switch len(servers) {
	case 0:
		fatal("[!] no iRUN servers found\n")
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

// scan probes every real /24 this machine can see for an iRUN banner,
// excluding this machine's own addresses.
func scan() []string {
	prefixes := autoDetectPrefixes()
	if len(prefixes) == 0 {
		fatal("[!] no subnet detected\n")
	}

	local := localIPs()
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
				if local[ip] {
					return
				}
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

// localIPs returns all IPv4 addresses assigned to this machine.
func localIPs() map[string]bool {
	out := map[string]bool{"127.0.0.1": true}
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
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
			out[ip4.String()] = true
		}
	}
	return out
}

// scanSideChannel probes every reachable /24 for an iRUN side channel
// (port 2223), excluding this machine's own addresses.
func scanSideChannel() []string {
	prefixes := autoDetectPrefixes()
	if len(prefixes) == 0 {
		fatal("[!] no subnet detected\n")
	}

	local := localIPs()
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
				if local[ip] {
					return
				}
				if isSideChannel(ip) {
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

// isSideChannel checks if a host has an iRUN side channel on port 2223.
func isSideChannel(ip string) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, sideChannelPort), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// sideChannelHost picks a single remote host for push/pull.
// If exactly one is found, returns it. If several are found,
// shows a numbered picker. Exits on none.
func sideChannelHost(remotes []string) string {
	switch len(remotes) {
	case 0:
		fatal("[!] no iRUN servers found\n")
	case 1:
		return remotes[0]
	default:
		fmt.Printf("[+] %d servers found:\n", len(remotes))
		for i, ip := range remotes {
			fmt.Printf("    %d) %s\n", i+1, ip)
		}
		return pick(remotes)
	}
	panic("unreachable")
}

// pushCmd uploads a local file to a remote machine via the side channel.
func pushCmd(local, remote string) {
	remotes := scanSideChannel()
	target := sideChannelHost(remotes)

	f, err := os.Open(local)
	if err != nil {
		fatal("[!] open %s: %v\n", local, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		fatal("[!] stat %s: %v\n", local, err)
	}

	u := fmt.Sprintf("http://%s:%d/push?path=%s", target, sideChannelPort, url.QueryEscape(remote))

	req, err := http.NewRequest(http.MethodPut, u, f)
	if err != nil {
		fatal("[!] request: %v\n", err)
	}
	req.ContentLength = fi.Size()

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		fatal("[!] push: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fatal("[!] push failed (%d): %s\n", resp.StatusCode, string(body))
	}

	fmt.Printf("[+] %s -> %s:%s\n", local, target, remote)
}

// pullCmd downloads a remote file from a remote machine via the side channel.
func pullCmd(remote, local string) {
	remotes := scanSideChannel()
	target := sideChannelHost(remotes)

	u := fmt.Sprintf("http://%s:%d/pull?path=%s", target, sideChannelPort, url.QueryEscape(remote))

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(u)
	if err != nil {
		fatal("[!] pull: %v\n", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fatal("[!] pull failed (%d): %s\n", resp.StatusCode, string(body))
	}

	f, err := os.Create(local)
	if err != nil {
		fatal("[!] create %s: %v\n", local, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		fatal("[!] write %s: %v\n", local, err)
	}

	fmt.Printf("[+] %s:%s -> %s\n", target, remote, local)
}

func pick(servers []string) string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Pick one (1-%d): ", len(servers))
		line, err := reader.ReadString('\n')
		if err != nil {
			fatal("[!] read error\n")
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
	if localIPs()[ip] {
		fatal("[!] refusing to connect to this machine (%s)\n", ip)
	}

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
		fatal("[!] connect: %v\n", err)
	}
	defer client.Close()

	sess, err := client.NewSession()
	if err != nil {
		fatal("[!] session: %v\n", err)
	}
	defer sess.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm", 120, 40, modes); err != nil {
		fatal("[!] pty: %v\n", err)
	}

	sess.Stdin = os.Stdin
	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr

	if err := sess.Shell(); err != nil {
		fatal("[!] shell: %v\n", err)
	}
	sess.Wait()
}
