// iRUN-find: probe every real /24 subnet this machine can see.
// Returns IPs of hosts listening on port 2222 (iRUN servers).
//
//	iRUN-find
//
// Cache: %USERPROFILE%\.irun\iRUN-servers.txt
package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

func main() {
	prefixes := autoDetectPrefixes()
	if len(prefixes) == 0 {
		fmt.Fprintln(os.Stderr, "[!] no subnet detected")
		os.Exit(2)
	}
	for _, p := range prefixes {
		fmt.Printf("[*] scanning %s.0/24 for port 2222 ...\n", p)
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
				conn, err := net.DialTimeout("tcp", ip+":2222", 500*time.Millisecond)
				if err == nil {
					conn.Close()
					mu.Lock()
					found = append(found, ip)
					mu.Unlock()
				}
			}(prefix, i)
		}
	}
	wg.Wait()
	sort.Strings(found)

	if len(found) == 0 {
		fmt.Fprintln(os.Stderr, "[!] no iRUN servers found")
		os.Exit(1)
	}

	ts := time.Now().Unix()
	dir := cacheDir()
	_ = os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "iRUN-servers.txt")
	f, err := os.Create(path)
	if err == nil {
		for _, ip := range found {
			name := ip
			if addrs, err := net.LookupAddr(ip); err == nil && len(addrs) > 0 {
				name = strings.TrimSuffix(addrs[0], ".")
			}
			fmt.Fprintf(f, "%s %s %d\n", ip, name, ts)
		}
		f.Close()
		fmt.Printf("[+] cache: %s\n", path)
	}

	fmt.Printf("[+] %d iRUN server(s) found:\n", len(found))
	for _, ip := range found {
		fmt.Printf("    %s\n", ip)
	}
}

func cacheDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".irun")
	}
	return os.TempDir()
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
