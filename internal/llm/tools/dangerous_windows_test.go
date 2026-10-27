package tools

// Gap #8: the Windows commands a wrong yes cannot take back. Test names end
// in a lowercase token because `go test -run` compiles each top-level
// alternative of the goal's pattern on its own.

import "testing"

func TestWindowsDangerousCommands_dangerous(t *testing.T) {
	rows := map[string]string{
		`Remove-Item HKLM:\SOFTWARE\Vendor -Recurse`:                "delete-registry",
		`Remove-Item -Path "HKCU:\Software\Vendor" -Recurse -Force`: "delete-registry",
		`Remove-ItemProperty -Path HKCU:\Software\X -Name Y`:        "delete-registry",
		`rm -r HKCU:\Software\Vendor`:                               "delete-registry",
		`Remove-Item Registry::HKEY_LOCAL_MACHINE\SOFTWARE\X`:       "delete-registry",
		`reg delete HKCU\Software\Vendor /f`:                        "delete-registry",
		`vssadmin delete shadows /all /quiet`:                       "delete-backups",
		`vssadmin.exe Delete Shadows /For=C:`:                       "delete-backups",
		`wbadmin delete catalog -quiet`:                             "delete-backups",
		`wmic shadowcopy delete`:                                    "delete-backups",
		`Get-CimInstance Win32_ShadowCopy | Remove-CimInstance`:     "delete-backups",
		`cipher /w:C:\`:         "wipe-free-space",
		`cipher.exe /W:D:\data`: "wipe-free-space",
		`Set-ExecutionPolicy Unrestricted -Scope LocalMachine`:                                      "execution-policy",
		`takeown /f C:\Windows\System32\drivers\etc\hosts`:                                          "take-ownership",
		`icacls C:\Windows\System32 /grant Everyone:F /T`:                                           "system-permissions",
		`icacls "C:\Program Files\App" /reset /T`:                                                   "system-permissions",
		`icacls C:\ /grant Users:F`:                                                                 "system-permissions",
		`icacls $env:ProgramData\X /setowner me`:                                                    "system-permissions",
		`sc delete WinDefend`:                                                                       "delete-service",
		`sc.exe \\server delete Spooler`:                                                            "delete-service",
		`Stop-Service -Name WinDefend -Force`:                                                       "stop-critical-service",
		`net stop wuauserv`:                                                                         "stop-critical-service",
		`Set-Service -Name EventLog -StartupType Disabled`:                                          "stop-critical-service",
		`schtasks /create /tn x /tr C:\x.exe /sc onlogon`:                                           "scheduled-task",
		`Register-ScheduledTask -TaskName x -Action $a`:                                             "scheduled-task",
		`reg add HKCU\Software\Microsoft\Windows\CurrentVersion\Run /v x /d C:\x.exe`:               "autorun-key",
		`Set-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\RunOnce -Name x -Value y`: "autorun-key",
		`New-ItemProperty -Path "HKCU:\Software\Microsoft\Windows\CurrentVersion\Run" -Name x`:      "autorun-key",
		`robocopy "C:\SourceDir" "D:\DestDir" /MIR /MT:16 /Z`:                                       "robocopy-deletes",
		`robocopy C:\a D:\b /PURGE`:                                                                 "robocopy-deletes",
		`robocopy C:\a D:\b *.log /MOVE`:                                                            "robocopy-deletes",
		`robocopy.exe C:\a D:\b /mov /mt:128`:                                                       "robocopy-deletes",
	}
	for cmd, want := range rows {
		d := DangerousPatternIn(cmd)
		if d == nil {
			t.Errorf("%q is not flagged; want %s", cmd, want)
			continue
		}
		if d.Name != want {
			t.Errorf("%q flagged as %s; want %s", cmd, d.Name, want)
		}
	}

	// Ordinary work must not be flagged: a warning on everything is a
	// warning nobody reads.
	benign := []string{
		`Get-ItemProperty HKLM:\Software\Microsoft\Windows\CurrentVersion\Run`,
		`Get-ChildItem HKCU:\Software`,
		`Remove-Item .\build -Recurse`,
		`robocopy C:\a D:\b /E /MT:64`,
		`robocopy C:\a D:\b /E /L`,
		`cipher /e secret.txt`,
		`icacls .\report.txt /grant Users:R`,
		`Get-Service WinDefend`,
		`Get-ScheduledTask`,
		`Stop-Service -Name MyDevServer`,
		`sc query WinDefend`,
		`Get-ExecutionPolicy -List`,
	}
	for _, cmd := range benign {
		if d := DangerousPatternIn(cmd); d != nil {
			t.Errorf("%q flagged as %s; it is ordinary work", cmd, d.Name)
		}
	}
}
