# Security

## Threat model

iRUN is a **trusted-LAN tool**. It binds `0.0.0.0:2222` and accepts any SSH
connection without authentication. That is the design, not a bug.

The intended environment is a private network you control (home, office,
lab). If the host has a public IP or is reachable from an untrusted
network, the server should not be running.

The bundled `iRUN-find` actively filters out link-local (`169.254.x.x`),
loopback, VMware, Hyper-V, and virtual adapters to avoid spamming
non-LAN ranges, but the server itself does not enforce a network boundary
beyond the host's own firewall configuration.

## What "no auth" actually means

- Anyone who can reach port 2222 can open a remote shell.
- Anyone with shell access can run any command as the user that started
  `iRUN.exe`.
- There is no audit log of session contents, only a banner log of commands
  received.
- The server has no rate limiting or brute-force surface because there is
  nothing to brute-force.

## Reporting a vulnerability

If you find a real vulnerability — for example, a way to break out of
the SSH protocol boundary, a remote code execution path triggered before
auth negotiation, or a leak of host secrets — please email
`security@` (or open a private security advisory on GitHub) instead of a
public issue.

Include a reproduction, the platform, and the iRUN version.
