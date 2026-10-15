<#
.SYNOPSIS
    Authenticode-signs build artifacts with the release certificate.

.DESCRIPTION
    Unsigned binaries that autostart a daemon and write under %APPDATA% attract
    SmartScreen warnings and antivirus suspicion on their own. Signing is the
    single highest-leverage change for distribution trust.

    The certificate is supplied as a base64-encoded PFX through the
    SIGNING_CERT_PFX_BASE64 secret rather than imported into the machine store,
    so nothing persists on the runner.

    This script fails rather than skipping. A release that quietly ships
    unsigned artifacts is worse than one that does not ship at all, because the
    absence of a signature is not visible to the person installing it.

.PARAMETER Targets
    Files to sign. Each must exist.

.EXAMPLE
    ./scripts/sign.ps1 -Targets focusd.exe, focusd_daemon.exe
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string[]] $Targets,

    [string] $TimestampUrl = $env:SIGNING_TIMESTAMP_URL
)

$ErrorActionPreference = 'Stop'

if (-not $TimestampUrl) {
    # RFC 3161 timestamp keeps the signature valid after the certificate expires.
    $TimestampUrl = 'http://timestamp.digicert.com'
}

function Get-SignTool {
    <#
        Locate signtool. The Windows SDK is not present on a stock GitHub runner,
        so the Visual Studio build tools are searched as well.
    #>
    $candidates = @(
        (Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\bin')
    )

    $signTool = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($signTool) {
        return $signTool.Source
    }

    foreach ($root in $candidates) {
        if (-not (Test-Path $root)) { continue }
        $found = Get-ChildItem -Path $root -Filter 'signtool.exe' -Recurse `
            -ErrorAction SilentlyContinue |
            Where-Object { $_.FullName -match '\\x64\\' } |
            Sort-Object -Property @{ Expression = {
                # Pick the newest SDK. A plain string sort ranks 10.0.9999.0 above
                # 10.0.10000.0, so the version is read from the path as a number.
                $m = [regex]::Match($_.FullName, '(\d+\.\d+\.\d+\.\d+)')
                if ($m.Success) { [version]$m.Groups[1].Value } else { [version]'0.0.0.0' }
            }; Descending = $true } |
            Select-Object -First 1
        if ($found) { return $found.FullName }
    }

    throw 'signtool.exe not found. Install the Windows SDK or run this on a runner that has it.'
}

function Get-SigningCertificatePath {
    <#
        Decode the PFX secret to a short-lived file and return its path.
    #>
    $base64 = $env:SIGNING_CERT_PFX_BASE64
    if (-not $base64) {
        throw @'
SIGNING_CERT_PFX_BASE64 is not set.

Every release must be signed, so this job will not continue. Add the certificate
as a repository secret: a base64-encoded .pfx file.

  [Convert]::ToBase64String([IO.File]::ReadAllBytes('cert.pfx'))
'@
    }

    $path = Join-Path $env:RUNNER_TEMP "focusd-signing-$PID.pfx"
    # Clear any file left by an aborted earlier step before writing, so a stale
    # certificate cannot be signed with by mistake.
    Remove-Item $path -Force -ErrorAction SilentlyContinue
    [IO.File]::WriteAllBytes($path, [Convert]::FromBase64String($base64.Trim()))

    return $path
}

$signTool = Get-SignTool
$certPath = Get-SigningCertificatePath

try {
    foreach ($target in $Targets) {
        if (-not (Test-Path $target)) {
            throw "Cannot sign '$target': file does not exist."
        }

        Write-Host "Signing $target"

        & $signTool sign `
            /f $certPath `
            /p $env:SIGNING_CERT_PASSWORD `
            /fd SHA256 `
            /td SHA256 `
            /tr $TimestampUrl `
            /v $target

        if ($LASTEXITCODE -ne 0) {
            throw "signtool failed for '$target' with exit code $LASTEXITCODE."
        }

        # Confirm the signature is actually present and chains to a trusted root,
        # rather than trusting the exit code alone.
        & $signTool verify /pa /v $target
        if ($LASTEXITCODE -ne 0) {
            throw "Signature verification failed for '$target'."
        }

        Write-Host "Signed and verified: $target"
    }
}
finally {
    Remove-Item $certPath -Force -ErrorAction SilentlyContinue
}

Write-Host 'All targets signed.'