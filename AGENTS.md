# AGENTS.md

A short, repo-scoped guide for coding agents working on iRUN. Keep this
file small and current — every line costs context.

## What this repo is

Three Go binaries that together provide a zero-auth SSH server, a LAN
scanner, and a matching SSH client. No auth, no config, no flags beyond
the bare minimum. The full intent lives in `docs/REQUIREMENTS.md` — read
it before changing behavior.

## Layout

```
main.go                  # iRUN (SSH server) — Windows is the primary target
find/main.go             # iRUN-find (LAN scanner) — pure Go, cross-platform
sshr/main.go             # sshr (SSH client)    — pure Go, cross-platform
igo/main.go              # igo (human connector) — pure Go, cross-platform
docs/REQUIREMENTS.md     # canonical behavior spec
Makefile                 # cross-platform dev entry: build, test, lint, clean
build.bat                # Windows convenience wrapper around `go build`
```

## Non-negotiables

- **No auth.** The server deliberately accepts the SSH `none` method. Do
  not add password handlers, key requirements, or "secure defaults" that
  change the wire behavior. The threat model is documented in `SECURITY.md`.
- **No CGo.** Server uses `cmd.exe` on Windows and Go's `os/exec`
  everywhere. No platform-specific shell wrappers.
- **No new flags without justification.** iRUN is intentionally flag-free.
- **Do not bump the embedded SSH host key** unless the protocol layer
  changes; clients rely on it being stable.

## Build, test, lint

```bash
make build      # bin/iRUN, bin/iRUN-find, bin/sshr, bin/igo
make lint       # gofmt -l + go vet
make test       # go test -race
```

CI runs the same on Linux, Windows, and macOS via
`.github/workflows/ci.yml`. Cross-OS release artifacts are produced by
`.github/workflows/release.yml` on `v*` tag pushes.

## Style

- `gofmt` clean. `go vet ./...` clean. No exceptions.
- Imports: stdlib first, then third-party, separated by a blank line.
  `goimports` ordering is fine.
- Errors: prefer `fmt.Errorf("...: %w", err)`. Surface to `os.Stderr`.
- Comments: explain *why*, not *what*. The code already says what.

## When you change behavior

1. Update `docs/REQUIREMENTS.md` first.
2. Update the matching `Usage` block in `README.md`.
3. Add an entry to the `[Unreleased]` section of `CHANGELOG.md`.
4. Add a test if the change is testable without a real network.

## Out of scope for this file

- Personal agent configuration (`~/.config/opencode/AGENTS.md`).
- Skill instructions for individual tools.
