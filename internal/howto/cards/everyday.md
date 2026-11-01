# everyday (read this before any web search)

The common jobs, with the tool that is already here. The "Helpers here" block of the prompt lists which exist on this machine.

    files tool, action=delete, path=old.txt        # delete: goes to the trash, never a permanent delete
    files tool, action=move, path=a.txt, dest=sub  # move or rename; action=list shows a folder
    view tool on app.log                           # read a file (bounded); the find tool searches names and text
    Get-Content app.log -Tail 30 -Wait             # follow a log (PowerShell); tail -n 30 -f app.log in bash
    Get-Content f.json -Raw | ConvertFrom-Json     # JSON in PowerShell; jq '.key' f.json when jq is listed
    Get-FileHash f.zip -Algorithm SHA256           # hash; sha256sum f.zip in bash
    magick in.png out.webp                         # convert a picture (ffmpeg -i in.png out.webp if no magick)
    tesseract scan.png out                         # OCR: text goes to out.txt, only when tesseract is listed

When a name is not in the "Helpers here" block it is not installed: do not search the web for it, use the nearest one.
