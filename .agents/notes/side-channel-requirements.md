# Side channel — Requirements

REST server running on the **remote machine** alongside `iRUN.exe`. Lets the
agent run commands on the remote host without dealing with SSH shell escaping.

## What it does

1. Starts automatically when `iRUN.exe` starts.
2. Binds `0.0.0.0:2223` on the remote machine.
3. Exposes REST endpoints for command execution on the remote host.
4. The local `igo.exe` client starts nothing — it only connects to the remote.
5. Agent calls the remote side channel directly over the LAN.

## Endpoints

### Health

```
GET /health
```

Returns `{"ok": true}`.

### Exec

Run a raw command string on the remote machine without SSH re-parsing.

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

- No auth needed (trusted LAN, same threat model as iRUN SSH).
- Zero user-facing flags or config.
- Must bypass PowerShell/cmd escaping hell for the agent.
- Lifetime tied to the parent `iRUN.exe` process.
- The local `igo.exe` must not run any server.
