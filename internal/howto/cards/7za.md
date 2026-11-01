# 7za (embedded toolbox: 7-Zip)

Makes and unpacks .7z, and also reads .zip, .tar, .gz, .rar. The `archive` tool does zip only: for a .7z use this.
Call it by name in the shell; extract with `x` (keeps folders), list with `l`.

    7za a docs.7z .\docs                 # make docs.7z from the folder docs
    7za x docs.7z -oout                  # extract into the folder out (no space after -o)
    7za l docs.7z                        # list what is inside
