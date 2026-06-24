# Plan: Restore Linux-OpenSSH exec semantics in iRUN

Created: 2026-06-25
Status: approved (CEO said "let's try it")

## Problem (one paragraph)

iRUN's exec mode today goes through `s.Command()` → gliderlabs' shlex split → direct `exec.Command(args[0], args[1:]...)`. This pre-parses the command with a POSIX shell parser and then never invokes a shell, breaking `&&`, `|`, `>`, `^`, `(`, `)`, etc. on Windows. The previous fix (skip `cmd /c`, go direct) traded one bug for the opposite one. We need the Linux-sshd contract: hand the **raw** string the client sent to `cmd.exe /c` and let the remote shell parse it.

## Changes

### 1. Server: `main.go`, exec branch of `shellHandler`

Replace lines 81-101 (the entire `if args := s.Command(); len(args) > 0 { ... }` block) with a single path that uses `s.RawCommand()` and runs `cmd /c`.

Before (current code at `main.go:81-101`):
```go
if args := s.Command(); len(args) > 0 {
    // Log every command received.
    fmt.Printf("  [%s] $ %s\n", s.User(), strings.Join(args, " "))
    // Exec mode: spawn directly via Go, no cmd.exe intermediary.
    // shlex.Split already parsed the client's quoting correctly,
    // so 'apt-get install -y htop' is already one arg.
    var cmd *exec.Cmd
    if args[0] == "wsl" {
        // wsl -d Ubuntu -- apt-get install -y htop
        // Go passes each arg as a separate OS argument (no shell re-parsing).
        cmd = exec.Command("wsl", args[1:]...)
    } else {
        cmd = exec.Command(args[0], args[1:]...)
    }
    cmd.Stdout = s
    cmd.Stderr = s
    cmd.Stdin = strings.NewReader("")
    _ = cmd.Run()
    return
}
```

After:
```go
if raw := s.RawCommand(); raw != "" {
    // Exec mode: hand the raw string the client sent to cmd.exe /c.
    // The remote shell parses the command. iRUN is a dumb pipe.
    // (OpenSSH sshd does the same with `bash -c <raw>`.)
    fmt.Printf("  [%s] $ %s\n", s.User(), raw)
    cmd := exec.Command("cmd.exe", "/c", raw)
    cmd.Stdout = s
    cmd.Stderr = s
    _ = cmd.Run()
    return
}
```

Notes:
- `s.RawCommand()` returns the exact bytes the client sent on the exec channel. Confirmed in `gliderlabs/ssh@v0.3.8/session.go:197`. This is the *original* string, not shlex's re-parse.
- Delete the `wsl` special case. `cmd /c wsl -d Ubuntu -- apt-get install -y htop` works because cmd.exe finds `wsl.exe` and passes the rest as its argv.
- `cmd.Stdin = strings.NewReader("")` is deleted — without a stdin, the OS attaches a null device, which is what we want for one-shot exec.
- The `strings` import is no longer used in this block. Check if it's used elsewhere in `main.go`; if not, remove it. (It is — `s.User()` and the log line use it. Keep the import.)

### 2. Client: `sshr/main.go`

**No code changes.** `sshr` already sends `os.Args[2]` as a single string to `sess.Run()`. That string lands on the server untouched. The fix is server-side only.

### 3. Docs

#### `README.md` — "Exec vs Shell: how it works" table (lines 155-167)

Update the Exec row to remove "no `cmd.exe /c` intermediary" wording. The new wording: "Args are passed to `cmd.exe /c` as a single string (just like OpenSSH runs `bash -c <command>`); the remote shell parses the command."

#### `CHANGELOG.md` — add to `[Unreleased]`

```
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
  instead of always reporting 0. `ssh user@host "exit 7"` now sees 7 in
  `$?` / `$LASTEXITCODE` on the client, matching the OpenSSH contract.
```

#### `docs/REQUIREMENTS.md` — add one bullet under Exec mode (line 32 area)

Add a note that nested double-quote commands routed through `cmd /c` suffer cmd.exe's quote-stripping, and that users should reach for `powershell -EncodedCommand <base64>` or `-File script.ps1` when that bites. This is a documentation of a downstream limitation, not a defect in iRUN.

### 4. Server change, addendum A: exit code propagation

In the same `shellHandler` exec branch, capture the exit code and call `s.Exit(code)`:

```go
if raw := s.RawCommand(); raw != "" {
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
```

`docs/REQUIREMENTS.md` line 32 already says `cmd /c <command>`. The code was the deviation; we are snapping back. **No change to that line** — but we are adding the nested-quote note described above.

## Success criteria

**Round 1 (verified by Maat, all PASS except 5 and 8):**

1. `sshr u@host "whoami"` → prints user, exit 0 — **PASS**
2. `sshr u@host "echo a && echo b"` → prints `a\nb`, exit 0 — **PASS**
3. `sshr u@host "dir C:\ | findstr Users"` → pipe works — **PASS**
4. `sshr u@host "false || echo rescued"` → prints `rescued`, exit 0 — **PASS**
5. `sshr u@host "exit 7"`; in parent shell `$LASTEXITCODE` → 7 — **FAIL: pre-existing bug, see Addendum A**
6. `sshr u@host "go version"` → multi-word args work — **PASS**
7. `sshr u@host "wsl --status"` → wsl still works — **PASS**
8. `sshr u@host "powershell -Command Get-Date"` (no inner quotes) → PowerShell opt-in works — **PASS** (test 8 was constructed wrong originally, see Addendum B)
9. `sshr u@host` (no command) → interactive shell mode — **PASS**
10. `scp` / `sftp` → SFTP subsystem path untouched — **SKIP, static check sufficient**

**Round 2 (verified by Maat, all PASS):**

- 5  `exit 7` → sshr exit 7 — **PASS**
- 5b `exit 42` → sshr exit 42 — **PASS** (not hardcoded)
- 5c `cmd /c exit 3` → sshr exit 3 — **PASS** (nested propagation)
- 8  `powershell -Command Get-Date` → date in stdout — **PASS**
- 8b `powershell -Command "Write-Output hello"` → `hello` substring in stdout — **PASS**
- Sanity 1, 2, 4, 6 → all still PASS (no regression)

### Addendum A: exit code propagation (real bug, fix it)

The current `shellHandler` does `_ = cmd.Run()` — discards both the error and the exit code. Gliderlabs then auto-sends exit 0 on channel close. So even though `cmd /c exit 7` returns 7 internally, sshr sees 0.

This is a pre-existing defect (the old shlex + direct-exec code had the same `_ = cmd.Run()`), not introduced by this diff. But it's the same family of "exec mode must behave like OpenSSH," and it's a one-line fix, so it goes in this release.

Fix: capture the exit code, call `s.Exit(code)` before returning.

### Addendum B: cmd.exe nested-quote limitation (document it, don't fix it)

When a user runs `sshr u@host "powershell -Command \"Get-Date\""`, the chain is:

- local PowerShell argv: `["sshr", "u@host", "powershell -Command \"Get-Date\""]` (3 elements, last is one string)
- sshr sends the third arg as the exec command string: `powershell -Command "Get-Date"`
- iRUN does `cmd /c powershell -Command "Get-Date"`
- cmd.exe strips the inner double quotes per its rules: PowerShell receives `-Command "Get-Date"` (as a string-literal expression)
- PowerShell evaluates the string literal `"Get-Date"` and prints the literal text `Get-Date`, not the date.

This is cmd.exe's quote-stripping rule, not iRUN's. On Linux bash, the equivalent would work because bash preserves quote state correctly. There's no iRUN-side fix that would make cmd.exe handle nested quotes the way bash does. The user's options are:

- `powershell -Command Get-Date` (no inner quotes — works for single-word commands)
- `powershell -EncodedCommand <base64>` (avoid all quoting, the original "trick" the user mentioned earlier)
- For genuinely complex PowerShell, write a `.ps1` file and run it: `powershell -File script.ps1`

This goes in `docs/REQUIREMENTS.md` as a single bullet under the Exec mode section so future readers know.

## Out of scope

- No new flag (`--cmd`, `--pwsh`, etc.). cmd.exe is the default; opt into other shells by naming them in the command.
- No env-var shell selection. We don't need it; cmd.exe is the universal Windows default.
- No version detection. cmd.exe is on every Windows install since NT 3.1.
- No PTY in exec mode. PTY stays exclusive to interactive shell mode.
- No changes to sshr client. The fix is server-side.

## Rollout

- Single commit: `main.go` (exec branch — both `cmd /c` and exit code propagation) + `README.md` + `CHANGELOG.md` + `docs/REQUIREMENTS.md` (nested-quote note)
- `go vet ./...` clean
- `gofmt -l` clean
- All binaries build
- Maat re-verifies tests 5 and 8 (with corrected `powershell -Command Get-Date` no-quote invocation)
- CI green (Linux, Windows, macOS)
- Tag a patch release (v0.1.1) after verification passes
