# robocopy (Windows, built in)

Prefer the files tool: it runs robocopy with a /L preview first, lists what would be copied AND deleted, asks,
and reads the exit code for you. Use robocopy in the shell only for a switch the files tool does not offer.

`robocopy SOURCE DEST [FILES...] [switches]` copies the CONTENTS of SOURCE into DEST (no trailing backslash on
either path, quote paths with spaces). FILES are name filters (default `*.*`).

## Which files
- `/S` subfolders, not empty ones; `/E` subfolders including empty ones; `/LEV:n` only n levels deep
- `/XF name|*.tmp ...` exclude files; `/XD dir ...` exclude folders (names or full paths)
- `/XO` skip files older than the destination copy; `/XN` skip newer; `/XC` skip changed; `/XX` leave extra destination files alone (not listed, not purged)
- `/MAXAGE:n` / `/MINAGE:n` days (or YYYYMMDD); `/MAX:bytes` / `/MIN:bytes`
- `/A` only files with Archive set; `/M` same, then clears it; `/IA:RASHCNETO` / `/XA:...` include/exclude by attribute
- `/XJ` do not follow junctions (use it on profile folders: AppData has junction loops)

## What gets deleted (permanent, no Recycle Bin)
- `/MIR` mirror: `/E` plus `/PURGE`. Deletes from DEST whatever is not in SOURCE.
- `/PURGE` deletes DEST files and folders no longer in SOURCE (without /E it only looks at the top level).
- `/MOVE` deletes files AND folders from SOURCE after they copied.
- `/MOV` deletes files only from SOURCE after they copied; folders stay.
Always run the same command with `/L` first (list only: nothing is copied or deleted) and show the user the
"*EXTRA File" lines: those are the deletions.

## How
- `/MT:n` n threads, 1-128 (default 8 when /MT is given without n). The prompt says how many logical CPUs this
  machine has; past that, more threads help only on slow links or many small files. /MT cannot be combined with /IPG or /EFSRAW.
- `/Z` restartable mode (survives a dropped network link, slower); `/B` backup mode (needs admin: reads files the
  ACLs deny); `/ZB` restartable, falling back to backup mode
- `/COPY:DATSOU` what to copy: D data, A attributes, T timestamps, S security (ACLs), O owner, U auditing. Default
  `/COPY:DAT`. `/COPYALL` = `/COPY:DATSOU`. `/SEC` = `/COPY:DATS`.
- `/DCOPY:DAT` the same for folders (default `/DCOPY:DA`; T keeps folder timestamps)
- `/R:n` retries (default 1 million!) and `/W:n` seconds between retries (default 30): set `/R:1 /W:1`
- `/L` list only; `/NP` no percentage; `/NFL` `/NDL` no file/dir lines; `/NJH` `/NJS` no header/summary;
  `/LOG:file` / `/LOG+:file` write the log to a file; `/TEE` also to the console; `/UNILOG:file` Unicode log
- `/J` unbuffered I/O, faster for huge files; `/COMPRESS` SMB compression on the network

## Exit code: a bit mask, NOT 0 = success
- 0 nothing to copy, already in sync
- 1 files copied
- 2 extra files or folders found in DEST (deleted with /MIR or /PURGE)
- 4 mismatched files or folders
- 8 some files or folders could not be copied (after the retries)
- 16 fatal: nothing copied (bad path, no access)
Codes add up: 3 = copied + extras. 0-7 is success; 8 or more is a failure.
In PowerShell: `robocopy ...; if ($LASTEXITCODE -ge 8) { "failed: $LASTEXITCODE" }`.

## Examples
    robocopy "C:\src" "D:\dst" /E /MT:32 /R:1 /W:1 /NP          # copy everything, 32 threads
    robocopy "C:\src" "D:\dst" /MIR /MT:64 /Z /L                 # preview a mirror (deletes nothing)
    robocopy "C:\src" "D:\dst" *.log /MOV /S                    # move only the .log files
    robocopy "C:\src" "D:\dst" /E /XD node_modules .git /XF *.tmp
