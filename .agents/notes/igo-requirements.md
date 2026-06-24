# igo — Requirements

Single local EXE. Human interactive SSH login only.

## What it does

1. Scan LAN for iRUN servers on port 2222.
2. If exactly one found: connect immediately.
3. If multiple found: print numbered list, ask user to type a number, connect to the chosen one.
4. If none found: exit with error.
5. Once connected: spawn interactive PTY shell on the remote machine.

## UX

```
> igo
[*] scanning...
[+] 1 server found: 192.168.1.42
[+] connecting to 192.168.1.42...
Microsoft Windows [Version ...]
C:\Users\foo>
```

```
> igo
[*] scanning...
[+] 3 servers found:
    1) 192.168.1.42
    2) 192.168.1.78
    3) 192.168.1.201
Pick one (1-3): 2
[+] connecting to 192.168.1.78...
...
```

## Constraints

- Single EXE, zero flags, zero config.
- PTY shell only. No exec mode. No file transfer. No other features.
- Does absolutely nothing else.
- Use current Windows username (`%USERNAME%`) when connecting.
- No host key prompts, no auth prompts.
- Must work against iRUN server (zero auth) only.
