# iRUN

[![CI](https://github.com/lirrensi/irun/actions/workflows/ci.yml/badge.svg)](https://github.com/lirrensi/irun/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/lirrensi/irun)](https://github.com/lirrensi/irun/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)
[![Go](https://img.shields.io/badge/go-1.26%2B-00ADD8.svg)](./go.mod)

Zero-auth SSH server + LAN scanner + native SSH client. Three binaries. Zero dependencies. Zero prompts. Zero config.

## Why this exists

SSH on Windows is painful. The built-in `ssh.exe` always asks for a password. There's no quick way to get a shell on another machine without keys, configs, or passwords. Every existing tool adds layers: `paramiko`, `sshpass`, config files, key generation...

iRUN strips all of that away. Run the server, connect from any SSH client on the planet — no credentials, no prompts, no friction. Close the window and it disappears. Nothing persists.

## Use cases

- **Your personal fleet.** Laptop, desktop, test machine, that old box in the corner — run iRUN on all of them. No pairing, no key management, no matching. You just run the app and login from any device. Manage your entire local fleet without a second thought.
- **Quick administration.** You show up at a friend's place. Drop an EXE. Run it. You now have a shell. Set up anything without clicking through 1000 UIs. Need to install something? SSH in. Need to check logs? SSH in. Done.
- **Remote setup.** WSL on a remote machine? Install it via SSH. Services, configs, debugging — all from one terminal. No RDP, no TeamViewer, no screen sharing.
- **Development.** Run iRUN on your dev box, connect from your laptop. Full terminal access for builds, deploys, testing. Zero setup every time.

## ⚠️ WARNING

**iRUN binds on `0.0.0.0:2222`.** That's every network interface. If your machine is internet-facing, anyone on the planet can connect. There is no auth. There is no password. There is no key.

**Never use on a real-facing server or suffer.** This is a LAN tool. Private network only. If you put this on a VPS with a public IP, you deserve what happens next.

## The three binaries

### iRUN — SSH server

A single-binary SSH server for Windows. Zero auth handlers means the SSH protocol's "none" authentication is accepted by default.

**Usage:**
```
iRUN.exe
```

That's it. It prints a banner, opens port 2222, and waits.

**What happens:**
- Any SSH client connects without credentials
- Exec mode (`ssh user@host "command"`) → runs the command directly, returns output
- Shell mode (`ssh user@host`) → interactive `cmd.exe`
- SCP/SFTP works (SFTP subsystem is registered)
- Server logs every command received: `[user] $ command here`
- Firewall rule auto-added on private profile (port 2222)
- Close the window → server stops, everything cleans up

**Elevation and the firewall rule**

`iRUN.exe` tries to install a Windows Firewall rule for TCP/2222 on the
**Private** profile so the server is reachable from your LAN. That call
needs Administrator, so you have two ways to run:

- **One-time admin, then non-admin forever.** Launch `iRUN.exe` elevated
  once (right-click → *Run as administrator*). The firewall rule is
  created and persists across reboots and across non-elevated launches.
  From then on, start `iRUN.exe` as your normal user. The server binds
  port 2222 fine because the firewall is already open, and the `cmd.exe`
  sessions it spawns for SSH are **not** elevated — `sshr user@host
  "whoami /groups"` will not show Administrator groups. Pick this mode
  if you want LAN access without handing SSH the keys to the kingdom.
- **Always admin.** Launch `iRUN.exe` elevated every time. The firewall
  rule is re-applied (deleted and re-added, idempotent) and every
  `cmd.exe` spawned for an SSH session inherits that elevation, so SSH
  behaves exactly like an elevated console. Pick this mode if you want
  SSH-in to have the same privileges as a privileged terminal.

The rule is named `iRUN SSH (port 2222, private profile only)` and is
recreated on every elevated launch, so re-running as admin is always
safe.

**Connect from any client:**
```bash
# Built-in ssh.exe
ssh -o PreferredAuthentications=none -p 2222 user@HOST

# PuTTY, WinSCP, FileZilla — just set port 2222, no password

# sshr (this project's client)
sshr user@HOST:2222 "whoami"
```

### iRUN-find — LAN scanner

Scans your local /24 subnet for machines running iRUN on port 2222.

**Usage:**
```
iRUN-find.exe
```

**What it does:**
- Auto-detects your local subnets (filters out VMware, Hyper-V, loopback, APIPA)
- 64 concurrent TCP dials, 500ms timeout per host
- Caches results at `%USERPROFILE%\.irun\iRUN-servers.txt`
- Prints found servers with username@ip

### sshr — SSH client

Native Go SSH client. Single binary, zero flags, no config.

**Usage:**
```
sshr USER@HOST[:2222] ["command"]
```

**What it does:**
- Exec mode: `sshr user@host:2222 "whoami"` → runs command, prints output, exits
- Shell mode: `sshr user@host` → interactive shell
- Auto-detects SSH keys (`~/.ssh/id_ed25519`, `id_rsa`, `id_ecdsa`)
- For iRUN servers: sends empty password + tries "none" auth automatically
- No prompts, no host key warnings, no config files

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  iRUN server (Windows)                              │
│                                                     │
│  SSH "none" auth → accepts all connections          │
│  Exec mode: Go os/exec → direct process spawn      │
│  Shell mode: cmd.exe → interactive                  │
│  SFTP subsystem → scp/sftp support                  │
│                                                     │
│  ┌─────────────────────────────────────────────┐    │
│  │  gliderlabs/ssh v0.3.8                      │    │
│  │  + pkg/sftp                                 │    │
│  │  + golang.org/x/crypto/ssh                  │    │
│  └─────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  sshr client (Go native)                            │
│                                                     │
│  golang.org/x/crypto/ssh → full SSH implementation  │
│  Auth: Password("") + "none" + auto-detect keys    │
│  Exec: session.Run() → one-shot                     │
│  Shell: session.Shell() → interactive PTY           │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│  iRUN-find (Go native)                              │
│                                                     │
│  net.Dial to port 2222 on every IP in /24           │
│  64 goroutines, 500ms timeout                       │
│  Filters: 169.254.x.x, 127.x.x.x, 172.16-31.x,    │
│           192.168.x.x, VMware/Hyper-V adapters     │
│  Cache: %USERPROFILE%\.irun\iRUN-servers.txt        │
└─────────────────────────────────────────────────────┘
```

### Exec vs Shell: how it works

The server distinguishes two modes exactly like OpenSSH:

| Mode    | Trigger                | What happens                                                                                          |
|---------|------------------------|-------------------------------------------------------------------------------------------------------|
| **Exec**  | Client sends a command | Args are passed to `exec.Command` directly (no `cmd.exe /c` intermediary, no shell re-quoting).       |
| **Shell** | Client requests a shell | `cmd.exe` spawned with a PTY, stdin/stdout/stderr bridged, interactive session.                      |

The key insight: Go's `os/exec` passes arguments directly to the OS without
shell re-parsing. When the client sends `apt-get install -y htop`, the args
arrive as a pre-parsed slice and `exec.Command` hands each one to the OS as
a separate argument. No quoting destruction.

## Building

Requires Go 1.26+ (the version pinned in [`go.mod`](./go.mod)).

### One-liner (Windows)

```bat
build.bat
```

This drops `iRUN.exe`, `iRUN-find.exe`, and `sshr.exe` in the repo root.

### Make (cross-platform)

```bash
make build      # produces bin/iRUN, bin/iRUN-find, bin/sshr (.exe on Windows)
make test       # go test -race
make lint       # gofmt + go vet
make run-scan   # build and run iRUN-find against your local /24
make clean
```

### Direct `go build`

```bash
go build -o iRUN.exe .
go build -o iRUN-find.exe ./find
go build -o sshr.exe ./sshr
```

## Dependencies

| Module                        | Version   | Purpose                       |
|-------------------------------|-----------|-------------------------------|
| `github.com/gliderlabs/ssh`   | v0.3.8    | SSH server framework          |
| `github.com/pkg/sftp`         | v1.13.10  | SFTP subsystem (for `scp`)    |
| `golang.org/x/crypto`         | v0.53.0   | SSH client implementation     |

All dependencies are Go modules — no system packages, no DLLs, no Python.

## Requirements

- Windows for the server (uses `cmd.exe` and Windows firewall APIs)
- The scanner and client are pure Go and run anywhere
- Go 1.26+ to build from source
- That's it

## Documentation

- [`docs/REQUIREMENTS.md`](./docs/REQUIREMENTS.md) — canonical behavior spec, end-to-end example, hard requirements
- [`CONTRIBUTING.md`](./CONTRIBUTING.md) — how to send a PR
- [`SECURITY.md`](./SECURITY.md) — threat model and disclosure
- [`AGENTS.md`](./AGENTS.md) — guidance for coding agents working in this repo
- [`CHANGELOG.md`](./CHANGELOG.md) — release history

## The philosophy

**Zero auth.** No passwords, no keys, no prompts. The server trusts your LAN. Close the window and it's gone.

**Zero deps.** Single binary per tool. No Python, no Node, no DLLs, no configs. Copy and run.

**Zero friction.** No flags, no config files, no host key warnings. `sshr user@host command` — that's the entire interface.

## License

[MIT](./LICENSE). Do whatever you want with it.
