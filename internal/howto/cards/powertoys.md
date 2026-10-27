# PowerToys (installed on this machine)

Microsoft PowerToys is a set of desktop utilities (github.com/microsoft/PowerToys). Most modules are driven from
the keyboard or Explorer, not from a shell: FancyZones (window layouts), PowerToys Run / Command Palette (launcher),
PowerRename (bulk rename in Explorer), Image Resizer, File Locksmith (which process holds a file), Peek (preview),
Text Extractor (OCR of the screen), Color Picker, Always On Top, Awake (keep the PC awake), Hosts File Editor,
Environment Variables, Registry Preview, Mouse utilities, Keyboard Manager.

## What you can run from the shell
The system prompt's PowerToys line lists the command-line programs found in THIS installation (read from its
files at startup, not from memory: modules gain and lose a CLI between releases). Before using one, read its own
help, for example:
    & "<install folder>\<program>.exe" --help

If nothing is listed, this installation has no command-line entry points: tell the user which module does the job
and how to open it (PowerToys Settings, or the module's shortcut), instead of inventing a command.

## Without PowerToys
- File Locksmith: Sysinternals `handle.exe <path>` if present, or `openfiles /query` (needs a setting turned on).
- PowerRename: the files tool's rename action (regex, templates, preview).
- Awake: `powercfg /requests` shows what keeps the machine awake; changing power plans is a system change, ask first.
