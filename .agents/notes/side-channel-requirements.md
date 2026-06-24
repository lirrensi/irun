# Side channel — Requirements

Local REST server embedded in `igo.exe` on the control machine. Lets the agent
run commands locally without dealing with Windows shell escaping.

## What it does

1. Starts automatically when `igo.exe` starts.
2. Binds a TCP port on localhost (first available in 4222-4299, then OS-assigned).
3. Exposes REST endpoints for local command execution.
4. User never interacts with it directly.
5. Agent discovers the port via `%USERPROFILE%\.irun\igo.port`.
6. Dies when `igo.exe` exits.

## Endpoints

### Health

```
GET /health
```

Returns `{"ok": true}`.

### Exec

Run a raw command string without local shell re-parsing.

```
POST /exec
Content-Type: application/json

{
  "shell": "cmd" | "powershell" | "pwsh",
  "command": "<any string>"
}
```

- `shell` defaults to `cmd` if omitted.
- Runs the string exactly as given by the agent in the chosen shell.
- Returns JSON with `stdout`, `stderr`, and `exit_code`.

## Constraints

- Binds localhost only (127.0.0.1).
- No auth needed.
- Zero user-facing flags or config.
- Must bypass PowerShell/cmd escaping hell for the agent.
- Lifetime tied to the parent `igo.exe` process.
