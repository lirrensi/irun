// sshr: one-shot SSH command runner. No password. No key prompt. No Python.
//
//	sshr USER@HOST[:2222] ["command"]
//	sshr USER@HOST[:2222]                    (interactive shell)
//	echo "command" | sshr USER@HOST[:2222]   (stdin pipe)
//
// With explicit command  → Exec mode, no PTY on remote, clean stdout.
// With piped stdin       → Exec mode, stdin is the command.
// Without either         → Shell mode, PTY, interactive.
//
// Auth: empty password (iRUN), auto-loaded ~/.ssh keys (regular servers).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, `usage: sshr USER@HOST[:2222] ["command"]`)
		fmt.Fprintln(os.Stderr, `       echo "command" | sshr USER@HOST[:2222]`)
		os.Exit(2)
	}

	user, host, port := splitTarget(os.Args[1])
	client := dial(user, host, port)
	defer client.Close()

	command, err := getCommand()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sshr: %v\n", err)
		os.Exit(1)
	}

	if command != "" {
		// Exec mode: one-shot command, no PTY
		sess, err := client.NewSession()
		if err != nil {
			fmt.Fprintf(os.Stderr, "sshr: %v\n", err)
			os.Exit(1)
		}
		defer sess.Close()
		sess.Stdout = os.Stdout
		sess.Stderr = os.Stderr
		if err := sess.Run(command); err != nil {
			if e, ok := err.(*ssh.ExitError); ok {
				os.Exit(e.ExitStatus())
			}
			fmt.Fprintf(os.Stderr, "sshr: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Shell mode: interactive
	sess, err := client.NewSession()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sshr: %v\n", err)
		os.Exit(1)
	}
	defer sess.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sess.RequestPty("xterm", 80, 40, modes); err != nil {
		fmt.Fprintf(os.Stderr, "sshr: pty: %v\n", err)
		os.Exit(1)
	}
	sess.Stdin = os.Stdin
	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr
	if err := sess.Shell(); err != nil {
		fmt.Fprintf(os.Stderr, "sshr: %v\n", err)
		os.Exit(1)
	}
	sess.Wait()
}

// getCommand returns the command to execute:
//   - explicit arg (os.Args[2]) if present
//   - reads from piped stdin if no arg and stdin is a pipe
//   - empty string means interactive shell
func getCommand() (string, error) {
	if len(os.Args) >= 3 {
		return os.Args[2], nil
	}

	// Check if stdin is a pipe / redirected file.
	fi, err := os.Stdin.Stat()
	if err != nil {
		return "", fmt.Errorf("stdin stat: %w", err)
	}
	if (fi.Mode() & os.ModeCharDevice) != 0 {
		return "", nil // terminal → interactive shell
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	cmd := strings.TrimRight(string(data), "\r\n")
	if cmd == "" {
		return "", fmt.Errorf("empty command from stdin")
	}
	return cmd, nil
}

// splitTarget parses "USER@HOST:PORT" into parts. PORT defaults to 2222.
func splitTarget(s string) (user, host string, port int) {
	port = 2222
	user = os.Getenv("USERNAME")
	if user == "" {
		user = "user"
	}
	if at := strings.LastIndex(s, "@"); at >= 0 {
		user, s = s[:at], s[at+1:]
	}
	if c := strings.LastIndex(s, ":"); c >= 0 {
		p, err := strconv.Atoi(s[c+1:])
		if err == nil && p > 0 && p < 65536 {
			port = p
			host = s[:c]
		} else {
			host = s
		}
	} else {
		host = s
	}
	return
}

// dial connects with empty password (iRUN) + auto-detected keys (regular servers).
func dial(user, host string, port int) *ssh.Client {
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods(),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sshr: %s: %v\n", addr, err)
		os.Exit(1)
	}
	return client
}

func authMethods() []ssh.AuthMethod {
	m := []ssh.AuthMethod{
		ssh.Password(""), // iRUN - empty password
		ssh.KeyboardInteractive(func(_, _ string, _ []string, _ []bool) ([]string, error) {
			return []string{""}, nil
		}),
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
			p := filepath.Join(home, ".ssh", name)
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			signer, err := ssh.ParsePrivateKey(data)
			if err != nil {
				continue
			}
			m = append(m, ssh.PublicKeys(signer))
		}
	}
	return m
}
