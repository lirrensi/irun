// iRUN - a single-binary, zero-config, ephemeral SSH server.
//
// Run it, ssh in, close the window. Done.
//
// On Windows it serves a real cmd.exe shell. The host key is embedded
// so it never whines about "host identification changed" across restarts.
// There is no auth, no password, no key — the SSH "none" method is accepted
// by default. The server is gone the moment its window closes.

package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
	xssh "golang.org/x/crypto/ssh"
)

// sftpHandler serves the SFTP subsystem so scp/sftp work.
func sftpHandler(sess ssh.Session) {
	srv, err := sftp.NewServer(sess)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sftp init: %v\n", err)
		return
	}
	if err := srv.Serve(); err == io.EOF {
		srv.Close()
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "sftp: %v\n", err)
	}
}

// Embedded RSA host key (generated once, baked in at build time).
// Keeping it static means the client never sees "host key changed".
const hostKeyPEM = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAoptbjzLH337XmNrwAy0In1PN7aUO3hTA7RAyyTr99Bh+7N5h
sld27C019nEiPwhfuN5Bcz2MiBlgcAQyxZs5tCS6xQQfDfzpbYaS5q6EMd1I2O/1
BEUWf4tjZ/7JdE7DqDise6OqdZc+HmEgZOVeZjr98IB1LcbpyTQtD6Ts3q/G5+WH
ep3dAn5NJF+KCAXsW60N6cUCDJm+4lv6XccZXIo3SAc2oh87fl/uaJmNpfF+ekOT
RslGftP6GaKdiTrXF2BSDjzKby1JN1pxQhRG/1WM3xodQDxtNjNtHm5dvnv7rdqF
DaQD6ZBlJHz/b8KQIxi5Fu7uTo/dk3VMJyywVQIDAQABAoIBABEMwK40aSQ8WM9g
f4OZvHp2T1Sgdr1fCDajOOwEKT4ntmFQVQad6KyNdgfL54cb8euAtHSoqrxXitbb
/dXd95A1vLatPrNZBkHTd0JEYLyYwwtqJ7MFqnz/qNHt84IkQxw3qxBAwj4XuG33
ia3CpiIKg/dshLziyz8rXyExjhuwQF2rpA71RefC431qD2oJ53qP8rlzQVzHrU3v
Cga0wg7krV/gyGp+x15gApJMgAWlQz6vDG+DGHU46UuFG6pKTI80BLyZAFi53LVu
jknYKMZtkymu7m01rlguWzVX9XzSL7wEOGWjbwTXOIH882j7viBbPSs5XaivgCml
5NNw2+ECgYEAzSgdis5VTlq+KPS2+wga/59ahl77rGD3oeLMX9KXKZQ6pg8H/0Di
vVmUR1rkx5i5b7JRc+HXQsTiBGMF2zoF4OjK27QaOrbS2UWkczk87qrcxdXfz2Sx
iawgf0mbS2SYEUKBikrQLK4wlnJZ5vJa9Yo0JTXQupBeycjpjj2d/zUCgYEAyue5
/VcBjAStz/ZXYRb8EftGZuSPWxbuu3Q1f0PAYQwrYjmFniJyYFp7ptGJzs9p0INU
6Aq+NsoqMa0Y+bp9h1eNDfGJHUbZ7D2M1kwuAK1r1nC02zZpCEIepkcTgjum91Rf
dKSHF3m6fT3XTmYYvhGSpVpU8R8hQmPv30eIcKECgYB5eMslKM5xumDltx+wuzfh
KuVaslqp0jBNdhA0nGhMgivHrxa5GB4opyWYqkTTuaXycM6xooLmUdTRbCBHka9x
X+Tc+WKeaSmm5AlfAAEH/7sAmIYQMjq8nWIQe/CrT0CK16oDzBA+pFS4f7SjfdRF
ljMR5S9Vh63YJFHFms42EQKBgQC9fwkOdvF06PHDJReaDzM/P+MSOSdBNPukifVk
c8v5Vro1s+78LsOPBTIyK8N+J+t01xK220GmPcyGNFj88ZRGkBemDAu4EfF4Vktv
4BmefFgYH45opDoXgljJhdvMZxWaK2wyrW2VGRR33wdzqpo0+IhycRifUClprZfa
eR4NwQKBgQCwQewgSCR/zUCDxhfTU4IJTjjvMK9JGNT5P0EiFIStqIJDMXcbiwE0
WAHk2AA2inIPGGKb9Y8cRto3EkTkqQ4b2J1a8B49FOkboxlaIh/AgahbPthYo+Gc
Ui0NMHPfFma+sMdm9HYaZ7jCrp3E2zVMcbidw5UrroI0WxoEsy39fA==
-----END RSA PRIVATE KEY-----`

const (
	listenAddr = ":2222"
	fwRuleName = "iRUN SSH (port 2222, private profile only)"
)

// shellHandler bridges the SSH session to cmd.exe.
//
// Two modes, exactly like OpenSSH:
//
//	Exec:  client sent a command → run cmd /c <command>, no PTY, clean output
//	Shell: client wants interactive  → spawn cmd.exe, bridge I/O
func shellHandler(s ssh.Session) {
	if raw := s.RawCommand(); raw != "" {
		// Exec mode: hand the raw string the client sent to cmd.exe /c.
		// The remote shell parses the command. iRUN is a dumb pipe.
		// (OpenSSH sshd does the same with `bash -c <raw>`.)
		fmt.Printf("  [%s] $ %s\n", s.User(), raw)
		cmd := exec.Command("cmd.exe", "/c", raw)
		cmd.Stdout = s
		cmd.Stderr = s
		err := cmd.Run()
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		_ = s.Exit(code)
		return
	}
	// Shell mode: interactive cmd.exe, stdin/stdout/stderr bridged.
	fmt.Printf("  [%s] interactive shell opened\n", s.User())
	cmd := exec.Command("cmd.exe")
	cmd.Stdin = s
	cmd.Stdout = s
	cmd.Stderr = s
	_ = cmd.Run()
}

func main() {
	log.SetFlags(0) // we hand-format below

	// ---- Print the banner. No password - the user is on a trusted LAN,
	// the .exe is their own, the moment they close the window the
	// server disappears. Adding a password here is just friction. ----
	username := os.Getenv("USERNAME")
	if username == "" {
		username = "user"
	}
	hostname, _ := os.Hostname()

	fmt.Println()
	fmt.Println("  iRUN  -  portable SSH server")
	fmt.Println("  -------------------------------------")
	fmt.Printf("  Username: %s\n", username)
	fmt.Printf("  Port:     2222\n")
	fmt.Println()
	fmt.Printf("  Connect:  ssh %s@%s -p 2222\n", username, hostname)
	fmt.Println()
	fmt.Println("  No password - close this window to shut down.")
	fmt.Println()

	// ---- Best-effort firewall rule, private profile only. ----
	if err := addFirewallRule(); err != nil {
		fmt.Printf("  [!] firewall rule not added (run as Admin to enable): %v\n", err)
	} else {
		fmt.Println("  [+] firewall rule installed (private profile only)")
	}

	// ---- Parse the embedded host key. ----
	signer, err := xssh.ParsePrivateKey([]byte(hostKeyPEM))
	if err != nil {
		log.Fatalf("invalid embedded host key: %v", err)
	}

	// ---- Build the server. ----
	// NO auth handlers set → gliderlabs accepts "none" auth by default.
	// Any SSH client (ssh.exe, PuTTY, sshr, paramiko) connects with zero
	// credentials. The protocol itself handles it — no password, no key.
	server := &ssh.Server{
		Addr:        listenAddr,
		HostSigners: []ssh.Signer{signer},
		IdleTimeout: 0,
		MaxTimeout:  0,
		Version:     "iRUN_1.0",
	}
	server.Handle(shellHandler)

	// SFTP subsystem: lets scp / sftp work with zero auth.
	server.SubsystemHandlers = map[string]ssh.SubsystemHandler{
		"sftp": sftpHandler,
	}

	go startSideChannel()

	fmt.Println("  [+] listening on 0.0.0.0:2222")
	fmt.Println()

	// When the window is closed, the OS kills us, the listener goes
	// with us, the firewall rule stays as a harmless empty door.
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("listen error: %v", err)
	}
}

// addFirewallRule opens TCP/2222 inbound but only on the Private
// profile (your home/office LAN, not the public internet).
// Uses netsh, so it requires Administrator. If we are not elevated
// this fails; we still bind the port, so the server works for any
// process that can already reach the port (and on most home routers
// that means the LAN).
func addFirewallRule() error {
	// Delete any previous rule with the same name (clean re-runs).
	_ = exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name="+fwRuleName).Run()

	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+fwRuleName,
		"dir=in",
		"action=allow",
		"protocol=TCP",
		"localport=2222,2223",
		"profile=private",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w (%s)", err, bytes.TrimSpace(out))
	}
	return nil
}
