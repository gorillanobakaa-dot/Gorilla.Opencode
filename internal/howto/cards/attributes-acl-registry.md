# Attributes, permissions (ACLs) and the registry: reading

## File attributes
    (Get-Item C:\f.txt).Attributes                      # e.g. ReadOnly, Archive
    Get-ChildItem C:\dir -Force | Select-Object Name, Attributes
    attrib C:\dir\*                                    # R read-only, H hidden, S system, A archive
Changing them: the files tool's attributes action (set/clear), or the powershell-files card.

## Who may do what (ACLs) - read only
    icacls C:\dir\f.txt                       # one line per account: (F) full, (M) modify, (RX) read+run, (R) read, (W) write
    icacls C:\dir /T /C /Q | Select-Object -First 40     # whole tree; /C keeps going past errors
    Get-Acl C:\dir\f.txt | Format-List Owner, AccessToString
    (I) means inherited from the parent; (OI)(CI) means it applies to files and subfolders below.
Changing an ACL (icacls /grant, /deny, /reset, takeown) is a security change: say exactly what it does and ask first.

## Registry - read only
    Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion' | Select-Object ProductName, DisplayVersion
    Get-ItemPropertyValue 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced' -Name HideFileExt
    Get-ChildItem 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall' | ForEach-Object { $_.GetValue('DisplayName') }
    reg query "HKLM\SYSTEM\CurrentControlSet\Control\FileSystem" /v LongPathsEnabled
HKLM is the machine (writing needs admin), HKCU the current user. Writing (Set-ItemProperty, reg add, reg delete)
changes how Windows or a program behaves: say what and why, and ask first. Export a key before changing it:
`reg export "HKCU\Software\X" C:\backup\x.reg`.
