@echo off
setlocal

echo [*] Building iRUN binaries ...

echo [1/4] iRUN.exe (SSH server)
go build -o iRUN.exe . || (echo [!] FAILED & exit /b 1)

echo [2/4] iRUN-find.exe (LAN scanner)
go build -o iRUN-find.exe ./find || (echo [!] FAILED & exit /b 1)

echo [3/4] sshr.exe (SSH client)
go build -o sshr.exe ./sshr || (echo [!] FAILED & exit /b 1)

echo [4/4] igo.exe (human iRUN connector)
go build -o igo.exe ./igo || (echo [!] FAILED & exit /b 1)

echo.
echo [+] All binaries built successfully:
echo     iRUN.exe        - SSH server
echo     iRUN-find.exe   - LAN scanner
echo     sshr.exe        - SSH client
echo     igo.exe         - human iRUN connector
