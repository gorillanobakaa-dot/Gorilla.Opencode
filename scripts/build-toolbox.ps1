# build-toolbox.ps1 - build gorilla-opencode.exe with every helper embedded (-tags toolbox).
#
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-toolbox.ps1 -HelpersDir <folder> [-NoCompress] [-Output <exe>]
#
# 1. builds pfind.exe from tools\pfind-rs (cargo, offline, from its committed vendor\ folder);
# 2. copies every other helper pinned for windows-amd64 in internal\toolbox\SHA256SUMS from -HelpersDir
#    (the from-source builds listed in third_party\PINS.md, "Embedded toolbox") into
#    internal\toolbox\bin\windows-amd64\, gzip-compressed (-NoCompress: raw);
# 3. stops if ANY helper's sha256 differs from its pin (nothing unpinned is ever embedded);
# 4. go build -tags toolbox, and prints the exe size.
# No network: cargo runs --offline, go uses the module cache. Run heavy steps under `fieldkit guard --` on a laptop.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$HelpersDir,
    [switch]$NoCompress,
    [string]$Output = "gorilla-opencode-toolbox.exe",
    [string]$Version = "dev"
)
$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
$sums = Join-Path $repo "internal\toolbox\SHA256SUMS"
$dest = Join-Path $repo "internal\toolbox\bin\windows-amd64"

$pins = @{}
foreach ($line in Get-Content -LiteralPath $sums) {
    $t = $line.Trim()
    if ($t -eq "" -or $t.StartsWith("#")) { continue }
    $f = $t -split "\s+"
    if ($f[1].StartsWith("windows-amd64/")) { $pins[$f[1].Substring(14)] = $f[0].ToLower() }
}
if ($pins.Count -eq 0) { throw "no windows-amd64 line in $sums" }

Write-Host "== pfind.exe: cargo build --release --offline --locked"
Push-Location (Join-Path $repo "tools\pfind-rs")
try {
    & cargo build --release --offline --locked
    if ($LASTEXITCODE -ne 0) { throw "cargo build failed ($LASTEXITCODE)" }
} finally { Pop-Location }
$pfind = Join-Path $repo "tools\pfind-rs\target\release\pfind.exe"

if (Test-Path -LiteralPath $dest) { Remove-Item -LiteralPath $dest -Recurse -Force }
New-Item -ItemType Directory -Path $dest | Out-Null

foreach ($name in ($pins.Keys | Sort-Object)) {
    $src = if ($name -eq "pfind.exe") { $pfind } else { Join-Path $HelpersDir $name }
    if (-not (Test-Path -LiteralPath $src)) { throw "$name is pinned but missing: $src" }
    $got = (Get-FileHash -Algorithm SHA256 -LiteralPath $src).Hash.ToLower()
    if ($got -ne $pins[$name]) {
        throw "$name has sha256 $got but SHA256SUMS pins $($pins[$name]); not embedding it (rebuilt pfind? update the pin and say why in the commit)"
    }
    if (-not $NoCompress) {
        $out = [IO.File]::Create((Join-Path $dest "$name.gz"))
        $gz = New-Object IO.Compression.GZipStream($out, [IO.Compression.CompressionLevel]::Optimal)
        $in = [IO.File]::OpenRead($src)
        try { $in.CopyTo($gz) } finally { $in.Dispose(); $gz.Dispose(); $out.Dispose() }
    } else {
        Copy-Item -LiteralPath $src -Destination (Join-Path $dest $name)
    }
    Write-Host ("  embedded {0,-14} {1,12:N0} bytes  sha256 {2}" -f $name, (Get-Item -LiteralPath $src).Length, $got)
}

Write-Host "== go build -tags toolbox"
Push-Location $repo
try {
    $env:CGO_ENABLED = "0"
    & go build -tags toolbox -trimpath -ldflags "-s -w -X github.com/opencode-ai/opencode/internal/version.Version=$Version" -o $Output .
    if ($LASTEXITCODE -ne 0) { throw "go build failed ($LASTEXITCODE)" }
    $exe = Get-Item -LiteralPath $Output
    Write-Host ("== {0}: {1:N0} bytes ({2:N1} MB)" -f $exe.FullName, $exe.Length, ($exe.Length / 1MB))
} finally { Pop-Location }
