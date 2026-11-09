# rg (embedded toolbox)

ripgrep: search the text inside files, recursively, fast. It skips what .gitignore skips and hidden files; add `-uu` to search those too.

    rg -n WORD DIR                  # lines containing WORD, with line numbers
    rg -l WORD DIR                  # only the names of the files that contain it
    rg -i -t go "func \w+Test" .    # case-insensitive, only Go files, regular expression
    rg --files -g "*.md" DIR        # list files by glob, no search
    rg --version
