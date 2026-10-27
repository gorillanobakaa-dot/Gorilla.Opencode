# PowerShell file operations

Prefer the files tool for copy, mirror, move, rename in bulk, attributes, details and delete: it previews, asks,
and deletes to the Recycle Bin. Use these cmdlets for one-off work or what the files tool does not do.
Remove-Item deletes FOR GOOD (no Recycle Bin). -WhatIf on any of these shows what would happen and does nothing.

## List and inspect (aliases: dir, ls, gci)
    Get-ChildItem -Path C:\Folder                      # files and folders
    Get-ChildItem -Path C:\Folder -Recurse -File       # files only, all subfolders (prefer the find tool)
    Get-ChildItem -Path C:\Folder -Directory           # folders only
    Get-ChildItem -Path C:\Folder -Force               # include hidden and system items (-Hidden: ONLY hidden)
    Get-Item C:\Folder\file.txt | Format-List *         # every property
    (Get-Item C:\Folder\file.txt).Attributes           # ReadOnly, Hidden, System, Archive ...
    Get-FileHash C:\Folder\file.txt -Algorithm SHA256

## Attributes
    Set-ItemProperty -Path C:\f.txt -Name Attributes -Value ReadOnly
    Set-ItemProperty -Path C:\f.txt -Name Attributes -Value Normal       # clears them all
    $f = Get-Item C:\f.txt; $f.IsReadOnly = $false
    $f.Attributes = $f.Attributes -bor [IO.FileAttributes]::Hidden     # add one, keep the rest
    attrib +r +h C:\f.txt    /    attrib -r C:\f.txt                    # the classic command

## Move, rename, copy
    Move-Item -Path C:\src\f.txt -Destination D:\dst\ -Force
    Rename-Item -Path C:\dir\old.txt -NewName new.txt
    Copy-Item -Path C:\src\f.txt -Destination D:\dst\f.txt
    Copy-Item -Path C:\srcdir -Destination D:\dstdir -Recurse

## Bulk rename (the files tool's rename action does this with a preview)
    Get-ChildItem -File | Rename-Item -NewName { $_.Name -replace 'oldText', 'newText' }
    $i = 1; Get-ChildItem -File | Rename-Item -NewName { "{0:D2}_{1}" -f $script:i++, $_.Name }

## Delete (permanent)
    Remove-Item -Path C:\dir\f.txt
    Remove-Item -Path C:\dir\sub -Recurse
    Remove-Item -Path C:\dir\* -Include *.tmp
    Remove-Item -Path C:\dir\locked.txt -Force            # also read-only and hidden files
To the Recycle Bin instead: the files tool's delete action.

## Create and write
    New-Item -ItemType Directory -Path C:\dir\new -Force
    Set-Content -Path C:\f.txt -Value 'text' -Encoding utf8
Windows PowerShell 5.1 writes UTF-8 WITH a BOM for -Encoding utf8; PowerShell 7 writes it without.
