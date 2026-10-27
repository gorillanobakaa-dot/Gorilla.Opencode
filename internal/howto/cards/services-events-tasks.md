# Services, event log and scheduled tasks: reading

All of these only read. Starting, stopping or reconfiguring a service, clearing a log, or changing a task changes
the machine: say what and why, and ask first (most need administrator rights).

## Services
    Get-Service | Where-Object Status -eq Running | Sort-Object DisplayName
    Get-Service -Name wuauserv, BITS | Format-List Name, Status, StartType
    Get-CimInstance Win32_Service -Filter "Name='wuauserv'" | Select-Object Name, State, StartMode, PathName
    sc.exe qc wuauserv                                  # configuration; `sc` alone is Set-Content in PowerShell

## Event log
    Get-WinEvent -LogName System -MaxEvents 30
    Get-WinEvent -FilterHashtable @{LogName='System'; Level=1,2; StartTime=(Get-Date).AddDays(-1)}   # critical + errors, last day
    Get-WinEvent -FilterHashtable @{LogName='Application'; ProviderName='Application Error'} -MaxEvents 10 | Format-List TimeCreated, Message
    Get-WinEvent -ListLog * | Where-Object RecordCount -gt 0 | Sort-Object RecordCount -Descending | Select-Object -First 20 LogName, RecordCount
Always pass -MaxEvents or a StartTime: a log holds hundreds of thousands of entries. Some logs (Security) need admin.

## Scheduled tasks
    Get-ScheduledTask | Where-Object State -ne Disabled | Select-Object TaskPath, TaskName, State
    Get-ScheduledTask -TaskName 'NAME' | Get-ScheduledTaskInfo          # last and next run, last result
    (Get-ScheduledTask -TaskName 'NAME').Actions                          # what it runs
    schtasks /query /fo LIST /v /tn "\Folder\NAME"
