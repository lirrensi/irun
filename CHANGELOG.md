# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `igo` — single EXE for human interactive login to iRUN servers. Scans the
  LAN, auto-connects if one server is found, asks for a number if multiple,
  then opens a PTY shell. Accepts a direct IP argument (`igo 192.168.66.78`).
  Ignores its own machine. On error the window pauses so the message can be
  read instead of closing instantly. Starts no servers.
- `iRUN` side channel — REST server (`POST /exec`) on port 2223 so the agent
  can run commands on the remote host without SSH escaping hell. The local
  `igo` client starts nothing.
- `igo push` / `igo pull` — file transfer through the side channel.
  `igo push <local> <remote>` uploads a file to the remote (creates parent
  directories automatically). `igo pull <remote> <local>` downloads a file.
  Remote is auto-discovered via LAN scan (same as shell mode). No
  `user@host:` notation, no flags, no config.
- `sshr` — exec mode via piped stdin. When no explicit command argument is
  given and stdin is a pipe/redirect, sshr reads the command from stdin.
  This lets you run commands on the remote without fighting PowerShell's
  quote-stripping:
  ```
  echo "choco install git -y" | sshr u@host
  ```
  Pipe mode passes the command to the remote shell verbatim — no `\"`,
  no `'` tricks, no nesting.

### Fixed
- **Exec mode: restore the OpenSSH contract.** The server now hands the
  raw command string to `cmd.exe /c` instead of pre-parsing it with
  gliderlabs' POSIX shlex and running `exec.Command` directly. This
  restores `&&`, `||`, `|`, `>`, `<`, `^`, and other cmd.exe operators
  that were silently dropped. Multi-word arguments (`apt-get install -y
  htop`) still work — the local shell quoting is unchanged; the remote
  shell parses the command, not the server. The `wsl` special case is
  gone; cmd.exe handles `wsl` natively as a binary.
- **Exec mode: propagate the remote command's exit code.** The server
  now reports the actual `cmd.exe` exit status back to the client
  instead of always reporting 0. `ssh user@host "exit 7"` now sees 7
  in `$?` / `$LASTEXITCODE` on the client, matching the OpenSSH
  contract.
- **I/O sliding timeout prevents connection hangs.** Every TCP
  connection is wrapped with an I/O deadline that auto-extends on
  every successful read/write. If no I/O happens for 5 minutes
  (e.g. crypto blocked by Windows Update during handshake), the
  connection is killed. Active sessions with flowing data never hit
  the limit because each packet extends the deadline. This replaces
  the previous IdleTimeout/MaxTimeout setup which couldn't catch
  crypto-blocked handshakes (the timeouts were checked inside Read()
  but blocked crypto doesn't reach Read()).

## [0.1.0] - 2026-06-25

First public release.

### Added
- `iRUN` — single-binary, zero-auth SSH server for Windows. Binds `0.0.0.0:2222`,
  supports Exec and Shell modes, registers the SFTP subsystem so `scp`/`sftp` work.
- `iRUN-find` — LAN scanner that probes every reachable /24 for port 2222 and
  caches results to `%USERPROFILE%\.irun\iRUN-servers.txt`.
- `sshr` — zero-prompt SSH client. Empty password for iRUN servers, automatic
  `~/.ssh/id_ed25519` / `id_rsa` / `id_ecdsa` for any standard SSH server.
- Cross-platform `Makefile` (`make build`, `make test`, `make lint`).
- GitHub Actions CI (format / vet / test / cross-platform build) and
  release workflow (cross-OS binaries on `v*` tag push).
