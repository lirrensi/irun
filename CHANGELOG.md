# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
