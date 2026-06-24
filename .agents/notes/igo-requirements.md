# igo — Requirements

Single local EXE. Human interactive SSH login only.

## What it does

1. Scan LAN for iRUN servers on port 2222.
2. Exclude this machine's own addresses.
3. If exactly one found: connect immediately.
4. If multiple found: print numbered list, ask user to type a number, connect to chosen one.
5. If none found: exit with error.
6. Once connected: spawn interactive PTY shell on the chosen server.

## Direct IP

```
igo 192.168.1.42
```

Connects directly to the given IP, skipping the scan.

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

- Single `.exe`. Zero flags. Zero config.
- PTY shell only. No exec mode. No file transfer. No side channel.
- Does absolutely nothing else.
- Use current Windows username (`%USERNAME%`) when connecting.
- No auth prompts, no host key prompts.
- Must never connect to its own machine.
