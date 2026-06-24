# Contributing

Thanks for your interest in iRUN.

## Ground rules

- **iRUN has no auth on purpose.** The server is designed for trusted LANs.
  Do not submit a PR that adds password prompts, key requirements, or
  "secure-by-default" warnings that change the runtime behavior. Docs and
  warnings are welcome; runtime gates are not.
- The server uses `cmd.exe` on Windows and Go's `os/exec` everywhere else.
  Do not introduce CGo or platform-specific shell wrappers.
- Keep it small. iRUN is a "no deps, no flags, no config" tool. New flags
  need a strong justification in the PR description.

## Process

1. Open an issue first for non-trivial changes. Small fixes and docs tweaks
   can go straight to PR.
2. Use the PR template. Mark which binary is touched.
3. Make sure `make lint` and `go test ./...` pass locally before pushing.
4. CI runs the same checks on Linux, Windows, and macOS — all three must pass.

## Development

```bash
make build      # builds all three binaries into bin/
make lint       # gofmt + go vet
make test       # go test -race
make run-scan   # builds and runs iRUN-find against the local subnet
make clean
```

## Commit style

Short subject line (50 chars or less), imperative mood, no trailing period.
Body explains the why, not the what.

## Reporting security issues

See `SECURITY.md`. Do not open a public issue for security reports.
