# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
