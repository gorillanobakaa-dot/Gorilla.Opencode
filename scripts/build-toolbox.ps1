# build-toolbox.ps1 - build gorilla-opencode.exe with every helper embedded (-tags toolbox).
#
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-toolbox.ps1 -HelpersDir <folder> [-NoCompress] [-Output <exe>]
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts\build-toolbox.ps1 -SignOnly <exe>    (step 5 alone)
#
# 1. builds pfind.exe from tools\pfind-rs (cargo, offline, from its committed vendor\ folder);
# 2. copies every other helper pinned for windows-amd64 in internal\toolbox\SHA256SUMS from -HelpersDir
#    (the from-source builds listed in third_party\PINS.md, "Embedded toolbox") into
#    internal\toolbox\bin\windows-amd64\, gzip-compressed (-NoCompress: raw);
# 3. stops if ANY helper's sha256 differs from its pin (nothing unpinned is ever embedded);
# 4. go build -tags toolbox, and prints the exe size;
# 5. Authenticode-signs the exe, ONLY when the owner's certificate is configured (gap #24, job 09):
#
#      GORILLA_SIGN_CERT_SHA1      thumbprint (40 hex) of a code-signing certificate with its private key in
#                                  Cert:\CurrentUser\My (a hardware token's certificate shows up there too).
#      GORILLA_SIGNTOOL            optional: full path of signtool.exe (Windows SDK). Default: the newest
#                                  C:\Program Files (x86)\Windows Kits\10\bin\<version>\x64\signtool.exe.
#      GORILLA_SIGN_TIMESTAMP_URL  optional: an RFC 3161 timestamp server. EMPTY BY DEFAULT ON PURPOSE: a timestamp
#                                  request sends the file's hash to that server (a third party learns what was
#                                  signed and when). Without one the signature stops validating when the
#                                  certificate expires.
#
#    Unset: "not signed", the exe untouched. Set: sign with SHA-256, then `signtool verify /pa`; any failure stops
#    the script, so a release that was meant to be signed never comes out unsigned (fail closed).
# No network: cargo runs --offline, go uses the module cache. Run heavy steps under `fieldkit guard --` on a laptop.
[CmdletBinding()]
param(
    [string]$HelpersDir,
    [switch]$NoCompress,
    [string]$Output = "gorilla-opencode-toolbox.exe",
    [string]$Version = "dev",
    [string]$SignOnly
)
$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot

function Invoke-ReleaseSigning([string]$File) {
    if (-not (Test-Path -LiteralPath $File)) { throw "no such file to sign: $File" }
    $thumb = "$env:GORILLA_SIGN_CERT_SHA1".Trim().Replace(" ", "")
    if ($thumb -eq "") {
        Write-Host "== not signed: GORILLA_SIGN_CERT_SHA1 is not set (see the header of scripts\build-toolbox.ps1)"
        return
    }
    if ($thumb -notmatch '^[0-9A-Fa-f]{40}$') { throw "GORILLA_SIGN_CERT_SHA1 is not a 40-hex thumbprint: $thumb" }
    $cert = Get-ChildItem -LiteralPath Cert:\CurrentUser\My | Where-Object { $_.Thumbprint -eq $thumb.ToUpper() }
    if (-not $cert) { throw "signing asked for, but certificate $thumb is not in Cert:\CurrentUser\My; the exe is NOT signed" }
    if (-not $cert.HasPrivateKey) { throw "certificate $thumb has no private key here; the exe is NOT signed" }
    if (-not ($cert.EnhancedKeyUsageList | Where-Object { $_.ObjectId -eq "1.3.6.1.5.5.7.3.3" })) {
        throw "certificate $thumb is not a code-signing certificate (EKU 1.3.6.1.5.5.7.3.3); the exe is NOT signed"
    }
    $signtool = "$env:GORILLA_SIGNTOOL"
    if ($signtool -eq "") {
        $kits = Join-Path ${env:ProgramFiles(x86)} "Windows Kits\10\bin"
        $found = Get-ChildItem -LiteralPath $kits -Directory -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -match '^\d+(\.\d+)+$' -and (Test-Path -LiteralPath (Join-Path $_.FullName "x64\signtool.exe")) } |
            Sort-Object { [version]$_.Name } | Select-Object -Last 1
        if ($found) { $signtool = Join-Path $found.FullName "x64\signtool.exe" }
    }
    if ($signtool -eq "" -or -not (Test-Path -LiteralPath $signtool)) {
        throw "signtool.exe not found (Windows SDK, or set GORILLA_SIGNTOOL); the exe is NOT signed"
    }
    $signArgs = @("sign", "/sha1", $thumb, "/s", "My", "/fd", "SHA256")
    if ("$env:GORILLA_SIGN_TIMESTAMP_URL" -ne "") { $signArgs += @("/tr", $env:GORILLA_SIGN_TIMESTAMP_URL, "/td", "SHA256") }
    & $signtool @signArgs $File
    if ($LASTEXITCODE -ne 0) { throw "signtool sign failed ($LASTEXITCODE); the exe is NOT signed" }
    & $signtool verify /pa $File
    if ($LASTEXITCODE -ne 0) { throw "signtool verify failed ($LASTEXITCODE)" }
    Write-Host "== signed: $File (certificate $thumb)"
}

if ($SignOnly) {
    Invoke-ReleaseSigning $SignOnly
    exit 0
}
if (-not $HelpersDir) { throw "-HelpersDir is required to build" }

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
    Invoke-ReleaseSigning $exe.FullName
} finally { Pop-Location }
