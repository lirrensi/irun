@echo off
setlocal

echo [*] Building iRUN binaries ...

echo [1/3] iRUN.exe (SSH server)
go build -o iRUN.exe . || (echo [!] FAILED & exit /b 1)

echo [2/3] iRUN-find.exe (LAN scanner)
go build -o iRUN-find.exe ./find || (echo [!] FAILED & exit /b 1)

echo [3/3] sshr.exe (SSH client)
go build -o sshr.exe ./sshr || (echo [!] FAILED & exit /b 1)

echo.
echo [+] All binaries built successfully:
echo     iRUN.exe        - SSH server
echo     iRUN-find.exe   - LAN scanner
echo     sshr.exe        - SSH client
