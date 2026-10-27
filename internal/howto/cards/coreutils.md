# coreutils (embedded toolbox: uutils, the GNU utilities in Rust)

One program holding the GNU utilities: call it as `coreutils <utility> <args>`. Useful in PowerShell for what has
no short cmdlet: wc, sort -u, uniq -c, cut, tr, seq, head -c, base64, sha256sum, numfmt, join, comm.
Do not use it to read or search files: the view and find tools do that, bounded.

    coreutils wc -l (Get-ChildItem -Recurse -Filter *.go).FullName   # line counts
    git log --format=%an | coreutils sort | coreutils uniq -c | coreutils sort -rn
    coreutils sha256sum release.zip
