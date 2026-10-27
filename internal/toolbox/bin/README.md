# internal/toolbox/bin

`go build -tags toolbox` embeds this folder. Only this README is committed: the helper programs are put in
`bin/<goos>-<goarch>/` (for example `bin/windows-amd64/sd.exe`, or `sd.exe.gz`) by `scripts/build-toolbox.ps1`
just before the release build, and every one of them must match its line in `../SHA256SUMS`.
