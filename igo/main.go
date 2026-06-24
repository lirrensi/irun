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
	"net"
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
