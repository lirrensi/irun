# iRUN — Requirements

The whole system: **drop iRUN.exe on the remote machine, run igo on your machine, get a shell. No password, no key, no Python, no config.**

## Principle of operation

1. Get the archive (`iRUN.exe`, `igo.exe`, `iRUN-find.exe`, `sshr.exe`).
2. Copy `iRUN.exe` to the remote machine — USB, cloud, whatever — and run it there. That machine becomes the server.
3. Run `igo.exe` on your machine. It finds the remote iRUN server and opens an interactive shell.
4. Use `iRUN-find.exe`, `sshr.exe`, and the side channel (`http://remote:2223`) so the agent can run commands on the remote machine the same way.

Remote machine = server. Your machine = client. USB is just transport.

---

## 1. `iRUN.exe` — the server

Single static `.exe`. Double-click → binds `0.0.0.0:2222` → prints banner → serves SSH-2.0. Close window → dead.

### Auth
**Zero auth.** Any connection with any username + empty password gets in. No keys. No prompt. No "none" dance.

### Two modes (exactly like OpenSSH on Linux)

#### Mode A: Shell (client sent no command)
```
ssh user@host -p 2222
```
- Client requests `Shell` + `PTY`
- Server spawns `cmd.exe` interactively
- Stdin/Stdout/Stderr bridged directly
- User gets a live shell. Types `dir`, gets output. Types `exit`, session closes.

#### Mode B: Exec (client sent a command)
```
ssh user@host -p 2222 whoami
```
- Client requests `Exec("whoami")`
- **NO PTY is created on the remote**
- **NO prompt is parsed. NO marker is inserted. NO filtering is done.**
- Server takes the command string, runs `cmd.exe /c <command>`
- Stdout and Stderr piped back, session closes when command exits
- Client prints the output. Done.

This is the standard SSH wire protocol. Must work against **any SSH server in existence**, including every Linux box.

### Hard requirements
- [ ] Single `.exe`, no DLLs, no config files
- [ ] Zero auth: empty password accepted always
- [ ] Shell mode: PTY + interactive `cmd.exe`
- [ ] Exec mode: no PTY, `cmd /c <command>`, stdout/stderr piped back, session closes
- [ ] Side channel: REST server on `0.0.0.0:2223` for agent command execution
  and file transfer (`POST /exec`, `PUT /push`, `GET /pull`)
- [ ] Note: nested double-quote commands routed through `cmd /c`
  suffer cmd.exe's quote-stripping rule (this is a cmd.exe
  limitation, not iRUN's). For complex PowerShell with embedded
  quotes, use `powershell -EncodedCommand <base64>` or write a
  `.ps1` and run `powershell -File script.ps1`.
- [ ] No firewall rule needed to function on LAN
- [ ] Window closed → port 2222 stops listening

---

## 2. `iRUN-find` — the network probe

Single `.exe`. Probes every /24 on this machine for port 2222 open. Returns IPs.

- [ ] Auto-detects LAN subnets (filters VMware, Hyper-V, loopback, APIPA)
- [ ] 64 concurrent TCP dials, 500ms timeout each
- [ ] Single `.exe`, zero deps
- [ ] Returns within ~10 seconds
- [ ] Output: one IP per line. Optional cache to `%USERPROFILE%\.irun\iRUN-servers.txt`

---

## 3. `sshr` — the runner (agent-only)

Single `.exe`. Zero dependencies. Behaves exactly like `ssh` on Linux.

Used by the agent, not the human.

### The only forms

```
sshr USER@HOST[:2222] ["command"]
echo "command" | sshr USER@HOST[:2222]
```

- With explicit command → **Exec mode**: no PTY, runs command, prints output
- With piped stdin        → **Exec mode**: stdin is the command (zero quoting)
- Without either          → **Shell mode**: PTY, interactive session

### Examples

```
sshr u@192.168.66.78:2222 whoami
echo "choco install git -y" | sshr u@192.168.66.78
sshr rx@DESKTOP-QENL7EU                  (interactive)
```

### Quote hell solved

PowerShell strips quotes when passing arguments to native .exes. The pipe
form bypasses this entirely — the command reaches the remote shell verbatim.
Use it whenever your command contains nested quotes, `&&`, `|`, `>`, or any
character that PowerShell would eat.

```
# Broken (PowerShell eats the quotes):
sshr u@host "echo ""hello"" & ver"

# Works (pipe is raw bytes):
"echo ""hello"" & ver" | sshr u@host
```

### Auth strategy (auto, zero prompts)
1. Empty password (works for iRUN servers)
2. `~/.ssh/id_ed25519`, `id_rsa`, `id_ecdsa` (picked up automatically, no flags)
3. Keyboard-interactive with empty response

### Hard requirements
- [ ] Single `.exe`. Zero dependencies. No Python, no `uv`, no `paramiko`, no `ssh.exe`
- [ ] **No password prompt, ever.** No key prompt, ever
- [ ] **Exec mode: NO PTY, NO prompt parsing, NO marker insertion, NO output filtering — the protocol handles it**
- [ ] Shell mode: PTY, interactive, stdin/stdout bridged
- [ ] Works against iRUN.exe AND any standard Linux OpenSSH server
- [ ] Returns within 5 seconds on LAN

---

## 4. `igo` — the human connector

Single `.exe` for humans only.

### Usage

```
igo
igo 192.168.66.78
igo push C:\local.txt C:\remote.txt
igo pull C:\remote.txt C:\local.txt
```

### What it does (interactive shell mode)
- Scans the LAN for iRUN servers on port 2222.
- If exactly one is found: connects immediately.
- If several are found: prints a numbered list and asks the user to type a number.
- Opens an interactive PTY shell on the chosen server.
- Never connects to its own machine.
- Starts no servers, no side channels.

### File transfer (push / pull)
- `igo push <local_path> <remote_path>` — uploads a file to the remote
  machine via the REST side channel (port 2223). Creates parent directories
  on the remote automatically.
- `igo pull <remote_path> <local_path>` — downloads a file from the remote
  machine via the REST side channel.
- Auto-detects the remote server the same way as shell mode (scan + pick
  if multiple). Both paths are plain local/remote filesystem paths — no
  `user@host:` SCP notation needed. The remote is found automatically.

### Hard requirements
- [ ] Single `.exe`. Zero flags. Zero config.
- [ ] Interactive shell: PTY only, no exec mode, no side channel.
- [ ] Push/pull: file transfer only via side channel HTTP API.
- [ ] Uses `%USERNAME%` to connect.
- [ ] No auth prompts, no host key prompts.
- [ ] Must never connect to its own machine.

---

## End-to-end

```
[double-click iRUN.exe]

sshr u@192.168.66.78:2222 whoami
→ desktop-xxx\u

sshr u@192.168.66.78:2222
→ interactive cmd.exe shell, type exit to quit
```

That's it. No flags. No parsing. No markers. No Python. SSH the way it was always meant to work.
