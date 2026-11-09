# fd (embedded toolbox)

Find files and folders by NAME (for text inside files use rg). Recursive, skips .gitignore entries and hidden files; add `-H -I` to include them.

    fd PATTERN DIR                  # names matching a regular expression
    fd -e go -e md DIR              # by extension
    fd -t d build DIR               # folders only (-t f files)
    fd -e log --changed-within 1d DIR   # modified in the last day
    fd --version
