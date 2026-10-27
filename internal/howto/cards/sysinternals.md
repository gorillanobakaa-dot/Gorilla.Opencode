# Sysinternals (found on PATH)

Microsoft's Sysinternals tools. The system prompt lists which ones are on PATH; use only those.
First run of each tool shows a license dialog that waits forever in this shell. `-accepteula` accepts the license
for the user (it writes a registry value under HKCU): ask the user once before adding it.

    handle.exe -accepteula -nobanner C:\path\file.txt    # which process has this file open (locked)
    sigcheck.exe -accepteula -nobanner -a C:\path\app.exe # signature, publisher, version
    autorunsc.exe -accepteula -nobanner -a * -ct          # everything that starts automatically, as CSV
    listdlls.exe -accepteula PROCESS                       # DLLs a process has loaded
    tcpvcon.exe -accepteula -a -c                          # network connections per process, CSV
    accesschk.exe -accepteula -nobanner USER C:\dir       # what one account may do there
    pslist.exe -accepteula                                 # processes with CPU and memory

Process Explorer (procexp) and Process Monitor (procmon) are windowed programs: start them for the user, they do not
print anything useful in this shell. pskill / psexec act on the machine: ask first.
