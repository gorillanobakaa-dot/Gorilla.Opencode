# jq (embedded toolbox)

Read, filter and edit JSON from the command line. It never edits in place: write to a temp file, then move it over.

    jq '.key' f.json                           # one field (-r drops the quotes on strings)
    jq '.items[] | select(.n > 3) | .name' f.json   # filter a list
    jq '.n = 7' f.json > t.json && mv t.json f.json  # change a value (use Move-Item on Windows)
    jq -s . a.json b.json                      # merge several files into one list
    jq --version
