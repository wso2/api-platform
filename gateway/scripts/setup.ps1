#Requires -Version 5.1
# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the
# License at http://www.apache.org/licenses/LICENSE-2.0
# --------------------------------------------------------------------
# Gateway quickstart setup (Windows). See README.md -> "Run".
# Windows counterpart of scripts/setup.sh; keeps the same flags, external
# tools (openssl + htpasswd/docker), and generated files.
#
# The server never auto-generates keys or certificates and has no demo/development
# mode: this script provisions everything the gateway needs, and the server fails
# closed with a descriptive error if a required key or certificate is missing.
#
# Provisions:
#   - listener-certs/default-listener.{crt,key}   : router HTTPS listener certificate
#   - aesgcm-keys/default-aesgcm256-v1.bin         : AES-256 at-rest encryption key. The gateway's
#       docker compose bind-mounts this host file into the controller.
#   - api-platform.env                            : required runtime defaults for the gateway-runtime
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File .\scripts\setup.ps1 [--force] [--certs-only]
#   pwsh -File ./scripts/setup.ps1 [--force] [--certs-only]

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# --- Move to the distribution root (scripts/setup.ps1 sits one level below docker-compose.yaml).
Set-Location -Path (Split-Path -Parent $MyInvocation.MyCommand.Path)
if (-not (Test-Path -LiteralPath 'docker-compose.yaml')) {
    Set-Location -Path '..'
}

$EnvFile = 'api-platform.env'

# Router downstream (HTTPS ingress) listener cert/key. Referenced by
# [router.downstream_tls] in config.toml as ./listener-certs/default-listener.{crt,key}
# and mounted into the gateway-controller. The repo checkout keeps these under
# gateway-controller/listener-certs; the distribution zip stages them under resources/.
if (Test-Path -LiteralPath 'gateway-controller/listener-certs' -PathType Container) {
    $CertsDir = 'gateway-controller/listener-certs'
} else {
    $CertsDir = 'resources/listener-certs'
}

# AES-256 at-rest encryption key.
if (Test-Path -LiteralPath 'gateway-controller' -PathType Container) {
    $EncKeyFile = 'gateway-controller/aesgcm-keys/default-aesgcm256-v1.bin'
} else {
    $EncKeyFile = 'resources/aesgcm-keys/default-aesgcm256-v1.bin'
}

# Controller config file - read only to detect whether basic auth is enabled,
# which determines whether admin credentials are required in api-platform.env.
$ConfigFile = 'configs/config.toml'

$Force = $false
$CertsOnly = $false

function Show-Usage {
    @'
Usage: .\scripts\setup.ps1 [--force] [--certs-only]

  --force        regenerate the certificate and encryption key (rotates them), rewrite api-platform.env,
                 and re-provision the admin credentials (rotates the password)
  --certs-only   generate only the listener TLS certificate (skip the encryption key and api-platform.env)

Admin credentials (gateway-controller REST/management API basic auth):
  Set ADMIN_USERNAME and/or ADMIN_PASSWORD in the environment to run non-interactively (CI).
  When unset and running interactively the script prompts; username defaults to "admin" and an empty
  password is randomly generated. Only the bcrypt hash is stored (in api-platform.env); the
  plaintext password is printed once and never written to disk.

The control-plane connection is optional and is NOT configured here: to connect to a control
plane, add APIP_GW_CONTROLLER_CONTROLPLANE_HOST and APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN to
api-platform.env by hand (both default to empty = standalone mode).
'@
}

foreach ($arg in $args) {
    switch ($arg) {
        '--force'      { $Force = $true }
        '--certs-only' { $CertsOnly = $true }
        { $_ -in @('-h', '--help', '/?') } { Show-Usage; exit 0 }
        default {
            [Console]::Error.WriteLine("unknown option: $arg (try --help)")
            exit 2
        }
    }
}

function Test-Command($name) {
    return [bool](Get-Command $name -ErrorAction SilentlyContinue)
}

function Write-Log($msg) { Write-Host "[setup] $msg" }

# Run openssl with its stderr chatter suppressed. openssl prints key-generation
# progress to stderr, and with $ErrorActionPreference='Stop' PowerShell turns any
# native stderr write into a terminating NativeCommandError - which 2>$null alone
# does not reliably suppress on Windows PowerShell 5.1. Relax the preference for
# the duration of the call and decide success from the exit code instead. The
# scriptblock reads script-scope variables ($CertsDir, $EncKeyFile) via the normal
# scope chain, so callers pass a bare `{ & openssl ... }`.
function Invoke-OpenSslQuiet([scriptblock]$Script, [string]$FailMessage) {
    $prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $out = & $Script 2>$null
    } finally {
        $ErrorActionPreference = $prev
    }
    if ($LASTEXITCODE -ne 0) { throw $FailMessage }
    return $out
}

# Is the Docker daemon actually reachable? The docker CLI being present does not
# mean the engine is running (Docker Desktop stopped, or in Windows-containers
# mode), and bcrypt hashing via the httpd image needs a live daemon. EAP is
# relaxed so a daemon-down stderr write does not throw before we read the exit code.
function Test-DockerDaemon {
    if (-not (Test-Command 'docker')) { return $false }
    $prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & docker info 2>&1 | Out-Null
        $ok = ($LASTEXITCODE -eq 0)
    } catch {
        $ok = $false
    } finally {
        $ErrorActionPreference = $prev
    }
    return $ok
}

# Verify the tools this script depends on before doing any work. openssl is a
# hard requirement for every path (cert, key, and password generation); the
# bcrypt tool (htpasswd or docker) is only needed when a new admin credential
# is generated, so its absence is a warning here and a hard failure later in
# Get-BcryptHash if and when hashing is actually attempted.
function Test-Prerequisites {
    Write-Log 'Verifying prerequisites ...'
    Write-Host "    [ok]      PowerShell $($PSVersionTable.PSVersion)"

    if (Test-Command 'openssl') {
        Write-Host "    [ok]      openssl - $(& openssl version 2>$null)"
    } else {
        Write-Host '    [MISSING] openssl'
        [Console]::Error.WriteLine('error: openssl is required (install it, or use Git for Windows / Docker Desktop, which ship it)')
        exit 1
    }

    if (Test-Command 'htpasswd') {
        Write-Host '    [ok]      htpasswd (used to bcrypt-hash the admin password)'
    } elseif (Test-Command 'docker') {
        if (Test-DockerDaemon) {
            Write-Host '    [ok]      docker (bcrypt-hashes the admin password via the httpd image)'
        } else {
            Write-Host '    [warn]    docker CLI found but its daemon is not reachable - start Docker Desktop (wait for the'
            Write-Host '              engine to be running), or install htpasswd (Apache httpd), before a new admin password'
            Write-Host '              can be hashed.'
        }
    } else {
        Write-Host '    [warn]    neither htpasswd nor docker found - only needed when a new admin password must be generated'
    }
    Write-Host ''
}

Test-Prerequisites

# Restrict a file to the current user only (Windows analogue of umask 177 / chmod 600).
# Uses icacls rather than Set-Acl of a freshly-built FileSecurity: the latter makes
# Set-Acl attempt to write the file's SACL, which requires the SeSecurityPrivilege a
# normal user does not hold. icacls only touches the DACL, so it needs no elevation.
# Best-effort: if permissions cannot be tightened, warn and continue rather than
# aborting setup over a hardening step (the file already lives under the user's
# profile, which is not world-readable by default).
function Set-OwnerOnlyAcl($path) {
    $prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $me = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
        # /inheritance:r drops inherited entries; /grant:r replaces the user's grant
        # with Full - together, only the current user is left on the file.
        & icacls $path /inheritance:r /grant:r "${me}:(F)" 2>&1 | Out-Null
        $code = $LASTEXITCODE
    } catch {
        $code = -1
    } finally {
        $ErrorActionPreference = $prev
    }
    if ($code -ne 0) {
        Write-Host "    [warn]    could not restrict permissions on $path - restrict it to your user manually if this host is shared."
    }
}

# bcrypt isn't in openssl; use htpasswd when available, else the httpd image.
# The gateway-controller's basic authenticator accepts bcrypt ($2a/$2b/$2y$) hashes.
# The password is written to the child process's stdin WITHOUT a trailing newline
# (mirroring bash's `printf '%s'`) - piping a PowerShell string would append CRLF
# and silently corrupt the hashed password.
#
# `argString` is passed to ProcessStartInfo.Arguments verbatim (not ArgumentList,
# which is unavailable on the .NET Framework that Windows PowerShell 5.1 runs on);
# it contains only static, script-controlled values - the password never appears
# here, only on stdin - so there is no argument-injection surface.
function Invoke-WithStdin($file, [string]$argString, [string]$inputText) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $file
    $psi.Arguments = $argString
    $psi.RedirectStandardInput = $true
    $psi.RedirectStandardOutput = $true
    $psi.RedirectStandardError = $true
    $psi.UseShellExecute = $false
    $proc = [System.Diagnostics.Process]::Start($psi)
    # Drain stdout/stderr asynchronously before blocking, so a chatty child
    # (e.g. docker pulling the httpd image to stderr) cannot fill a pipe buffer
    # and deadlock while we wait on the other stream.
    $outTask = $proc.StandardOutput.ReadToEndAsync()
    $errTask = $proc.StandardError.ReadToEndAsync()
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($inputText)
    $proc.StandardInput.BaseStream.Write($bytes, 0, $bytes.Length)
    $proc.StandardInput.BaseStream.Flush()
    $proc.StandardInput.Close()
    $proc.WaitForExit()
    if ($proc.ExitCode -ne 0) {
        throw "command '$file' failed (exit $($proc.ExitCode)): $($errTask.Result)"
    }
    return $outTask.Result
}

function Get-BcryptHash([string]$password) {
    if (Test-Command 'htpasswd') {
        $out = Invoke-WithStdin 'htpasswd' '-niB -C 10 ""' $password
    } elseif (Test-Command 'docker') {
        if (-not (Test-DockerDaemon)) {
            [Console]::Error.WriteLine('error: cannot bcrypt-hash the admin password - htpasswd is not installed and the Docker daemon is not reachable.')
            [Console]::Error.WriteLine('       Start Docker Desktop (wait until the engine is running), or install htpasswd (from Apache httpd), then re-run.')
            exit 1
        }
        $out = Invoke-WithStdin 'docker' 'run --rm -i httpd:2.4-alpine htpasswd -niB -C 10 ""' $password
    } else {
        [Console]::Error.WriteLine('error: need either htpasswd (from Apache httpd) or docker to bcrypt-hash the admin password')
        exit 1
    }
    # htpasswd emits "<user>:<hash>"; user is empty here. Take everything after the first colon.
    $line = ($out -split "`n" | Where-Object { $_ -match ':' } | Select-Object -First 1)
    $hash = $line.Substring($line.IndexOf(':') + 1)
    return $hash.Trim()
}

function New-ListenerCert {
    if (-not $Force -and (Test-Path -LiteralPath "$CertsDir/default-listener.crt") -and (Test-Path -LiteralPath "$CertsDir/default-listener.key")) {
        Write-Log "  - $CertsDir/default-listener.crt already exists - keeping it"
        return
    }
    New-Item -ItemType Directory -Force -Path $CertsDir | Out-Null
    Invoke-OpenSslQuiet {
        & openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes `
            -keyout "$CertsDir/default-listener.key" -out "$CertsDir/default-listener.crt" `
            -subj "/O=WSO2 API Platform/CN=localhost" `
            -addext "subjectAltName=DNS:localhost,DNS:*.localhost,DNS:host.docker.internal,IP:127.0.0.1"
    } "openssl failed to generate the listener certificate" | Out-Null
    Write-Log "  - self-signed listener certificate generated at $CertsDir/default-listener.crt"
}

function New-EncryptionKey {
    if (-not $Force -and (Test-Path -LiteralPath $EncKeyFile)) {
        Write-Log "  - $EncKeyFile already exists - keeping it"
        return
    }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $EncKeyFile) | Out-Null
    # openssl writes the file itself (-out) so PowerShell's text redirection cannot corrupt the raw bytes.
    Invoke-OpenSslQuiet { & openssl rand -out $EncKeyFile 32 } "openssl failed to generate the encryption key" | Out-Null
    Set-OwnerOnlyAcl $EncKeyFile
    Write-Log "  - AES-256 encryption key generated at $EncKeyFile"
}

# Report whether a required key exists (and is non-empty) in the kept env file,
# tallying $script:missing for any that are absent or empty. Mirrors setup.sh's
# check_env_key.
$script:missing = 0
function Test-EnvKey($key) {
    $line = @(Get-Content -LiteralPath $EnvFile |
        Where-Object { $_ -match "^\s*$([regex]::Escape($key))=" }) | Select-Object -Last 1
    if ($null -eq $line) {
        Write-Host "    [MISSING] $key"; $script:missing++
    } elseif (($line -replace '^[^=]*=', '') -eq '') {
        Write-Host "    [empty]   $key (defined but has no value)"; $script:missing++
    } else {
        Write-Host "    [ok]      $key"
    }
}

# Admin credentials are required only when controller.auth.basic is enabled.
# Detect that from config.toml so we don't demand them when basic auth is off.
# Mirrors setup.sh's basic_auth_enabled awk scan.
function Test-BasicAuthEnabled {
    if (-not (Test-Path -LiteralPath $ConfigFile)) { return $false }
    $inSection = $false
    foreach ($ln in Get-Content -LiteralPath $ConfigFile) {
        if ($ln -match '^\[controller\.auth\.basic\]\s*$') { $inSection = $true; continue }
        if ($ln -match '^\[') { $inSection = $false }
        if ($inSection -and $ln -match '^\s*enabled\s*=\s*true') { return $true }
    }
    return $false
}

Write-Log 'Provisioning listener TLS certificate ...'
New-ListenerCert

if ($CertsOnly) {
    exit 0
}

Write-Log 'Provisioning AES-256 encryption key ...'
New-EncryptionKey

if (-not $Force -and (Test-Path -LiteralPath $EnvFile)) {
    Write-Log "$EnvFile already exists - keeping it (rerun with --force to rewrite it)"

    # Since the env file is kept as-is (not regenerated), it may predate the
    # current gateway and be missing settings the runtime now requires. Check
    # each required key and refuse to report success if any are absent/empty.
    Write-Host ''
    Write-Log "Verify $EnvFile still defines the settings the gateway requires:"
    Test-EnvKey 'GATEWAY_CONTROLLER_HOST'
    Test-EnvKey 'LOG_LEVEL'
    if (Test-BasicAuthEnabled) {
        Write-Host "  Required because controller.auth.basic is enabled in ${ConfigFile}:"
        Test-EnvKey 'APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_USERNAME'
        Test-EnvKey 'APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH'
    }

    Write-Host ''
    if ($script:missing -gt 0) {
        Write-Log "$($script:missing) required setting(s) are missing or empty - setup is NOT complete."
        Write-Host ''
        Write-Host '  Before starting the gateway, either:'
        Write-Host "    - edit $EnvFile and fill in the value(s) flagged above, or"
        Write-Host '    - rerun to regenerate it from scratch:  powershell -ExecutionPolicy Bypass -File .\scripts\setup.ps1 --force'
        exit 1
    }

    Write-Log 'Setup complete.'
    Write-Host ''
    Write-Host '  Next step:'
    Write-Host '    docker compose up'
    exit 0
}

# Admin credentials for the gateway-controller REST/management API (basic auth).
# Precedence: ADMIN_USERNAME/ADMIN_PASSWORD env vars > interactive prompt > defaults.
# Only the bcrypt hash is persisted; the plaintext password is shown once at the end.
$generatedPassword = Invoke-OpenSslQuiet { & openssl rand -base64 24 } "openssl failed to generate a password"
$generatedPassword = ($generatedPassword -replace '[/+=]', '')
$generatedPassword = $generatedPassword.Substring(0, [Math]::Min(20, $generatedPassword.Length))

$interactive = -not [Console]::IsInputRedirected

$adminUsername = $env:ADMIN_USERNAME
if ([string]::IsNullOrEmpty($adminUsername) -and $interactive) {
    $adminUsername = Read-Host 'Admin username [admin]'
}
if ([string]::IsNullOrEmpty($adminUsername)) { $adminUsername = 'admin' }

$adminPassword = $env:ADMIN_PASSWORD
if ([string]::IsNullOrEmpty($adminPassword) -and $interactive) {
    $secure = Read-Host 'Admin password [press Enter to generate one]' -AsSecureString
    $bstr = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try {
        $adminPassword = [System.Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    } finally {
        [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    }
}
if ([string]::IsNullOrEmpty($adminPassword)) { $adminPassword = $generatedPassword }

Write-Log 'Provisioning admin credentials ...'
$adminPasswordHash = Get-BcryptHash $adminPassword
Write-Log '  - APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH generated (bcrypt)'

Write-Log "Writing $EnvFile ..."
$timestamp = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
# LF line endings + UTF-8 without BOM so docker-compose env_file parses cleanly.
$lines = @(
    "# Generated by scripts/setup.ps1 on $timestamp."
    '# The admin password is not stored here; it was printed once by scripts/setup.ps1.'
    '#'
    '# Required runtime settings - read directly by the gateway-runtime entrypoint / policy-engine:'
    'GATEWAY_CONTROLLER_HOST=gateway-controller'
    'LOG_LEVEL=info'
    ''
    "APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_USERNAME=$adminUsername"
    "APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH=$adminPasswordHash"
    ''
)
$content = ($lines -join "`n")
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText((Join-Path (Get-Location) $EnvFile), $content, $utf8NoBom)
Set-OwnerOnlyAcl $EnvFile
Write-Log "  - $EnvFile written"

Write-Host ''
Write-Log 'Setup complete.'
Write-Host ''
Write-Host '  ------------------------------------------------------------------'
Write-Host "   Gateway-controller admin login:  $adminUsername / $adminPassword"
Write-Host '   This password will not be shown again - copy it now.'
Write-Host "   (Only its bcrypt hash is stored, in $EnvFile)"
Write-Host '  ------------------------------------------------------------------'
Write-Host ''
Write-Host '  Next step:'
Write-Host '    docker compose up'
