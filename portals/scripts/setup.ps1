#Requires -Version 5.1
# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License. You may obtain a copy of the
# License at http://www.apache.org/licenses/LICENSE-2.0
# --------------------------------------------------------------------
#
# Shared quickstart setup script for both the AI Workspace and the Developer
# Portal quickstarts (and their standalone distribution zips) on Windows.
# Windows counterpart of scripts/setup.sh: same flags, same external tools
# (openssl + htpasswd/docker), same generated files.
#
# Both stacks run the same three services - platform-api, ai-workspace,
# developer-portal - behind docker-compose profiles, so this single script
# provisions everything either stack (or both at once) might need:
#
#   - a self-signed TLS certificate shared by all three services
#   - API Portal's own encryption key and session secret, written to
#     resources/keys/api-portal-encryption.key and api-portal-session-secret and
#     read by config.toml via {{ file }} - never stored as an env var
#   - the Platform API's at-rest encryption key, written to resources/keys/encryption.key
#     and read by config.toml via {{ file }} - like the JWT keypair below, never stored
#     as an env var
#   - an RS256 JWT signing keypair for the Platform API, written as PEM files
#     under resources/keys (jwt_private.pem / jwt_public.pem) and read by
#     config.toml via {{ file }} - tokens are signed asymmetrically, so there
#     is no shared HMAC secret to copy between services
#   - an admin username/password (prompted interactively - see below),
#     bcrypt-hashed into APIP_CP_ADMIN_USERNAME / APIP_CP_ADMIN_PASSWORD_HASH
#   - COMPOSE_PROJECT_NAME in .\.env, unique to this copy of the distribution.
#     Read by the docker compose CLI (not by the containers) to prefix every
#     container, network, and volume, so another extraction of the same zip on
#     this host cannot bind to this stack's volumes - the Platform API's
#     database and the Developer Portal's data.
#
# This is a ONE-TIME step. It never runs as part of container startup - every
# service fails closed at startup if a required secret is missing, rather
# than silently generating or accepting a weaker one. Re-running this script
# is safe: by default it only fills in what's missing and never overwrites an
# existing value; pass --force to rotate the TLS cert, JWT signing keypair,
# and admin credentials. --force deliberately does NOT touch the at-rest
# encryption key - see --rotate-encryption-key below.
#
# Usage (run from either portals\ai-workspace or portals\developer-portal, or
# from the root of a standalone distribution zip - same script, same layout):
#   powershell -ExecutionPolicy Bypass -File ..\scripts\setup.ps1     # inside the repo checkout
#   powershell -ExecutionPolicy Bypass -File .\scripts\setup.ps1      # inside a distribution zip
#   pwsh -File ../scripts/setup.ps1                                   # PowerShell 7+ is also fine
#   docker compose up -d   # uses this pack's default profiles, set via COMPOSE_PROFILES in .\.env
#   docker compose --profile <ai-workspace|developer-portal|all> up -d  # override
#
# Flags:
#   --force                   regenerate TLS cert, JWT signing keypair, and
#                              admin credentials (never the encryption key)
#   --certs-only              generate only the TLS certificate
#   --rotate-encryption-key   DESTRUCTIVE: replace an existing at-rest
#                              encryption key. Anything encrypted under the
#                              old key becomes permanently unreadable.
#   --profiles=<a,b,...>      override the default COMPOSE_PROFILES this
#                              script writes to .env (see DEFAULT_COMPOSE_PROFILES
#                              below)
#
# ADMIN_USERNAME / ADMIN_PASSWORD environment variables skip the interactive
# prompts and pin the credentials (used by CI). If ADMIN_PASSWORD is left
# unset at an interactive terminal, pressing Enter at the prompt generates a
# random one, printed once at the end.
#
# To rotate a single value by hand, delete it from api-platform.env (or
# delete resources\certificates / resources\keys) and re-run this script.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$Force = $false
$CertsOnly = $false
$RotateEncryptionKey = $false
$ProfilesOverride = ''

# The comma-separated COMPOSE_PROFILES value this script writes to .env, so
# that a plain `docker compose up -d` (no --profile flag) starts the right
# services for this pack. Each pack's `make dist` target bakes in its own
# value here when it copies this shared script into the distribution zip (see
# the `dist` target in portals/ai-workspace/Makefile and
# portals/developer-portal/Makefile). Left at this placeholder, it falls back
# to Get-Pack's topology-based detection below, which is what happens when
# this script runs straight out of a repo checkout (not a dist zip).
$DEFAULT_COMPOSE_PROFILES = '__DEFAULT_COMPOSE_PROFILES__'
# Which pack this is ("developer-portal" or "ai-workspace") - `make dist`
# bakes this in the same way as DEFAULT_COMPOSE_PROFILES above, so a shipped
# distribution zip never needs to detect it at runtime. Left at this
# placeholder, Get-Pack below determines it from docker-compose.yaml's actual
# service topology instead.
$DEFAULT_PACK_NAME = '__DEFAULT_PACK_NAME__'
# This pack's version, baked in by `make dist` the same way as the two values
# above. Only feeds the generated COMPOSE_PROJECT_NAME. Left at this
# placeholder, the VERSION file in a repo checkout is read instead.
$DEFAULT_DIST_VERSION = '__DEFAULT_DIST_VERSION__'

function Show-Usage {
    @'
Usage: .\scripts\setup.ps1 [--force] [--certs-only] [--rotate-encryption-key] [--profiles=<a,b,...>]

  --force                   regenerate TLS cert, JWT signing keypair, and
                             admin credentials. Never rotates the at-rest
                             encryption key on its own - see
                             --rotate-encryption-key.
  --certs-only              generate only the TLS certificate
  --rotate-encryption-key   DESTRUCTIVE: replace resources\keys\encryption.key
                             and resources\keys\api-portal-encryption.key even if
                             they already exist. Any data encrypted
                             under the old key(s) (e.g. stored secrets) becomes
                             permanently unreadable. Requires interactive
                             confirmation unless ADMIN_USERNAME/ADMIN_PASSWORD
                             are set (CI), in which case passing this flag is
                             itself treated as confirmation.
  --profiles=<a,b,...>      override the default COMPOSE_PROFILES value this
                             script writes to .env, e.g. --profiles=all or
                             --profiles=platform-api

ADMIN_USERNAME / ADMIN_PASSWORD environment variables skip the interactive
prompts and pin the credentials (used by CI).

Compose project name (data isolation):
  Setup writes COMPOSE_PROJECT_NAME into .env on the first run, unique to this copy of
  the distribution. It prefixes every container, network, and volume, so another
  extraction of this zip on the same host cannot bind to this stack's volumes (the
  Platform API's database, the Developer Portal's data).
  The name is pinned once and never changes afterwards - not on a rerun, not with
  --force, not with --rotate-encryption-key. A new name would leave the running data
  behind in the old volumes and start the stack with an empty database. Deleting .env
  has that same effect, so leave it in place.
  To choose the name yourself, set COMPOSE_PROJECT_NAME in the environment for the
  FIRST run:
    $env:COMPOSE_PROJECT_NAME = 'my-workspace'; .\scripts\setup.ps1
  It must match ^[a-z0-9][a-z0-9_-]*$ (no dots, no uppercase - docker compose rejects
  those).
  That is also the upgrade path from a distribution that had no .env: its data lives in
  volumes prefixed with the old extracted-folder name (find it with: docker volume ls),
  so pass that name on the first run to keep it.
'@
}

function Write-Log($msg) { Write-Host "[setup] $msg" }
function Invoke-Fail($msg) {
    [Console]::Error.WriteLine("[setup] ERROR: $msg")
    exit 1
}

# PowerShell's argument-mode parser splits an unquoted comma into an array, so
# a single $args element can itself be an array (@('--profiles=a', 'b')).
# Flatten it back to the bash form so both `--profiles=a,b` and
# `--profiles="a,b"` work.
foreach ($rawArg in $args) {
    $arg = if ($rawArg -is [array]) { ($rawArg -join ',') } else { [string]$rawArg }
    switch -Regex ($arg) {
        '^--force$'                 { $Force = $true; continue }
        '^--certs-only$'            { $CertsOnly = $true; continue }
        '^--rotate-encryption-key$' { $RotateEncryptionKey = $true; continue }
        '^--profiles=(.*)$'         { $ProfilesOverride = $Matches[1]; continue }
        '^(-h|--help|/\?)$'         { Show-Usage; exit 0 }
        default {
            [Console]::Error.WriteLine("[setup] ERROR: unknown option: $arg (try --help)")
            exit 2
        }
    }
}

$ThisDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# This script is invoked from a caller's working directory that has its own
# docker-compose.yaml one level below it (portals\ai-workspace, or
# portals\developer-portal, or the root of a distribution zip via
# scripts\setup.ps1) - never from this script's own directory. Look at the
# current directory first, then fall back to a sibling/parent of this file, so
# the same file works whether it's called as `..\scripts\setup.ps1` (dev
# checkout) or `.\scripts\setup.ps1` (distribution zip layout).
$cwd = (Get-Location).Path
if (Test-Path -LiteralPath (Join-Path $cwd 'docker-compose.yaml')) {
    $RootDir = $cwd
} elseif (Test-Path -LiteralPath (Join-Path $ThisDir 'docker-compose.yaml')) {
    $RootDir = $ThisDir
} elseif (Test-Path -LiteralPath (Join-Path $ThisDir '..\docker-compose.yaml')) {
    $RootDir = (Resolve-Path (Join-Path $ThisDir '..')).Path
} else {
    Invoke-Fail 'could not find docker-compose.yaml in the current directory, next to this script, or its parent directory. Run this from portals\ai-workspace, portals\developer-portal, or a distribution zip''s root.'
}
Set-Location -LiteralPath $RootDir

$EnvFile = Join-Path $RootDir 'api-platform.env'
# Project-level env file - Docker Compose reads COMPOSE_PROFILES from here
# automatically (unlike api-platform.env above, which is only ever passed to
# containers explicitly via each service's own env_file: entry), so this is
# what makes a plain `docker compose up -d` resolve the right services.
$ComposeEnvFile = Join-Path $RootDir '.env'
# COMPOSE_PROJECT_NAME goes in the same file. The full prefix is
# "$ProjectNamePrefix-$Pack", so the two packs never share a project name.
$ProjectNamePrefix = 'wso2apip'
$ProjectName = ''
$CertsDir = Join-Path $RootDir 'resources\certificates'
# RS256 JWT keypair (PEM) + at-rest encryption keys. Mounted into the
# platform-api container at /etc/platform-api/keys and into the api-portal
# container at /etc/api-portal/keys, read by config.toml via {{ file }}.
$KeysDir = Join-Path $RootDir 'resources\keys'
$ComposeFile = Join-Path $RootDir 'docker-compose.yaml'

# Every generated file is written as UTF-8 without a BOM and with LF line
# endings: api-platform.env / .env are parsed by the Docker CLI (a BOM or a
# stray CR would end up inside the first key's name or a value's trailing
# byte), and the key/secret files are read by config.toml's {{ file }} helper.
$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)
function Write-TextFileLf([string]$path, [string]$text) {
    [System.IO.File]::WriteAllText($path, ($text -replace "`r`n", "`n"), $Utf8NoBom)
}

function Test-CommandExists($name) {
    return [bool](Get-Command $name -ErrorAction SilentlyContinue)
}

# Restrict a file to the current user only (Windows analogue of chmod 600).
# icacls rather than Set-Acl: Set-Acl on a freshly-built FileSecurity tries to
# write the SACL, which needs a privilege a normal user does not hold.
#
# Unlike setup.sh, no container-UID entry is needed - Docker Desktop presents
# bind-mounted files to the container with permissive ownership regardless of
# the host DACL. Best-effort: warn and continue rather than aborting setup
# over a hardening step.
function Set-OwnerOnlyAcl([string]$path) {
    $prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $me = [System.Security.Principal.WindowsIdentity]::GetCurrent().Name
        # /inheritance:r drops inherited entries; /grant:r replaces the user's
        # grant with Full - together, only the current user is left on the file.
        & icacls $path /inheritance:r /grant:r "${me}:(F)" 2>&1 | Out-Null
        $code = $LASTEXITCODE
    } catch {
        $code = -1
    } finally {
        $ErrorActionPreference = $prev
    }
    if ($code -ne 0) {
        Write-Log "  - WARNING: could not restrict permissions on $path - restrict it to your user manually if this host is shared."
    }
}

# Run openssl with its stderr chatter suppressed and MSYS path conversion off,
# deciding success from the exit code instead.
#
# openssl writes key-generation progress to stderr, and under
# $ErrorActionPreference='Stop' any native stderr write becomes a terminating
# NativeCommandError that 2>$null alone does not reliably suppress on Windows
# PowerShell 5.1.
#
# Git for Windows ships the openssl most Windows hosts have, and its MSYS
# runtime rewrites POSIX-looking arguments into Windows paths - so
# `-subj "/O=WSO2 API Platform/CN=localhost"` would arrive as
# `C:/Program Files/Git/O=WSO2 API Platform/CN=localhost`.
function Invoke-OpenSslQuiet([scriptblock]$Script, [string]$FailMessage) {
    $prev = $ErrorActionPreference
    $prevNoPathConv = $env:MSYS_NO_PATHCONV
    $prevArgConvExcl = $env:MSYS2_ARG_CONV_EXCL
    $ErrorActionPreference = 'Continue'
    $env:MSYS_NO_PATHCONV = '1'
    $env:MSYS2_ARG_CONV_EXCL = '*'
    try {
        $out = & $Script 2>$null
    } finally {
        $ErrorActionPreference = $prev
        # Assigning $null removes the variable, restoring an originally-unset env.
        $env:MSYS_NO_PATHCONV = $prevNoPathConv
        $env:MSYS2_ARG_CONV_EXCL = $prevArgConvExcl
    }
    if ($LASTEXITCODE -ne 0) { Invoke-Fail $FailMessage }
    return $out
}

# The docker CLI being present does not mean the engine is running (Docker
# Desktop stopped, or in Windows-containers mode), and bcrypt hashing via the
# httpd image needs a live daemon.
function Test-DockerDaemon {
    if (-not (Test-CommandExists 'docker')) { return $false }
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

# openssl is a hard requirement for every path (cert, keys, password
# generation); the bcrypt tool (htpasswd or docker) is only needed when a new
# admin credential is generated, so its absence is a warning here and a hard
# failure later in Get-BcryptHash if hashing is actually attempted.
function Test-Prerequisites {
    Write-Log 'Verifying prerequisites ...'
    Write-Host "    [ok]      PowerShell $($PSVersionTable.PSVersion)"

    if (Test-CommandExists 'openssl') {
        # Routed through Invoke-OpenSslQuiet so a build that writes a benign
        # warning to stderr (e.g. "can't open config file") can't abort the
        # whole script on this version check.
        $opensslVersion = (Invoke-OpenSslQuiet { & openssl version } 'openssl is installed but failed to run')
        Write-Host "    [ok]      openssl - $(([string]$opensslVersion).Trim())"
    } else {
        Write-Host '    [MISSING] openssl'
        Invoke-Fail 'openssl is required (install it, or use Git for Windows / Docker Desktop, which ship it)'
    }

    if (Test-CommandExists 'htpasswd') {
        Write-Host '    [ok]      htpasswd (used to bcrypt-hash the admin password)'
    } elseif (Test-CommandExists 'docker') {
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

# Writes to the child's stdin WITHOUT a trailing newline (mirroring bash's
# `printf '%s'`) - piping a PowerShell string would append CRLF and silently
# corrupt the hashed password.
function Invoke-WithStdin([string]$file, [string]$argString, [string]$inputText) {
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
        Invoke-Fail "command '$file' failed (exit $($proc.ExitCode)): $($errTask.Result)"
    }
    return $outTask.Result
}

# bcrypt isn't in openssl; use htpasswd when available, else a throwaway httpd
# container. Unlike setup.sh, the username is deliberately NOT passed to the
# tool: bcrypt hashes are username-independent (htpasswd only uses it for the
# "<user>:<hash>" prefix stripped below), so keeping it off the command line
# removes any argument-injection surface from an interactively-typed username.
function Get-BcryptHash([string]$password) {
    if (Test-CommandExists 'htpasswd') {
        $out = Invoke-WithStdin 'htpasswd' '-niB -C 12 ""' $password
    } elseif (Test-CommandExists 'docker') {
        if (-not (Test-DockerDaemon)) {
            [Console]::Error.WriteLine('[setup] ERROR: cannot bcrypt-hash the admin password - htpasswd is not installed and the Docker daemon is not reachable.')
            [Console]::Error.WriteLine('        Start Docker Desktop (wait until the engine is running), or install htpasswd (from Apache httpd), then re-run.')
            exit 1
        }
        $out = Invoke-WithStdin 'docker' 'run --rm -i httpd:2.4-alpine htpasswd -niB -C 12 ""' $password
    } else {
        Invoke-Fail 'need either htpasswd (from Apache httpd) or docker to bcrypt-hash the admin password.'
    }
    # htpasswd emits "<user>:<hash>"; user is empty here. Take everything after the first colon.
    $line = ($out -split "`n" | Where-Object { $_ -match ':' } | Select-Object -First 1)
    if ($null -eq $line) {
        [Console]::Error.WriteLine('[setup] ERROR: could not parse a bcrypt hash from the password-hashing tool output.')
        [Console]::Error.WriteLine('        Ensure htpasswd (from Apache httpd) or a working Docker daemon is available, then re-run.')
        exit 1
    }
    return $line.Substring($line.IndexOf(':') + 1).Trim()
}

# Read an env file as LF-normalised lines, tolerating a file written with CRLF
# by an editor on a previous manual edit.
function Read-EnvLines([string]$file) {
    if (-not (Test-Path -LiteralPath $file)) { return @() }
    $text = [System.IO.File]::ReadAllText($file)
    if ([string]::IsNullOrEmpty($text)) { return @() }
    return @($text -split "`n" | ForEach-Object { $_.TrimEnd("`r") })
}

function Test-EnvKeySet([string]$file, [string]$key) {
    $pattern = "^$([regex]::Escape($key))="
    return [bool](@(Read-EnvLines $file | Where-Object { $_ -match $pattern }).Count)
}

# Sets KEY=VALUE in the given env file. Idempotent by default - never
# overwrites a value the user or a previous run already set - unless --force
# was passed, in which case any existing line for KEY is replaced.
function Set-EnvVar([string]$file, [string]$key, [string]$value) {
    $name = Split-Path -Leaf $file
    if (-not $Force -and (Test-EnvKeySet $file $key)) {
        Write-Log "  - ${key} already set in $name, leaving as-is"
        return
    }
    $pattern = "^$([regex]::Escape($key))="
    $lines = New-Object System.Collections.ArrayList
    foreach ($line in (Read-EnvLines $file)) {
        if ($line -notmatch $pattern) { [void]$lines.Add($line) }
    }
    # Drop the trailing empty element the final newline produces, so repeated
    # runs don't accumulate blank lines. An ArrayList rather than slicing:
    # $arr[0..($arr.Count - 2)] duplicates the sole element of a 1-item array,
    # because 0..-1 counts backwards.
    while ($lines.Count -gt 0 -and $lines[$lines.Count - 1] -eq '') {
        $lines.RemoveAt($lines.Count - 1)
    }
    [void]$lines.Add("$key=$value")
    Write-TextFileLf $file (($lines -join "`n") + "`n")
    Write-Log "  - ${key} set in $name"
}

# 32-byte secret as 64 lowercase hex chars, written LF-terminated (config.toml's
# {{ file }} helper trims trailing whitespace on read).
function New-HexSecretFile([string]$path) {
    $hex = (Invoke-OpenSslQuiet { & openssl rand -hex 32 } "openssl failed to generate a random secret for $path")
    $hex = ([string]$hex).Trim()
    if ($hex -notmatch '^[0-9a-f]{64}$') {
        Invoke-Fail "openssl produced an unexpected value for $path (expected 64 hex characters)."
    }
    Write-TextFileLf $path ($hex + "`n")
    Set-OwnerOnlyAcl $path
}

# One-time confirmation gate for --rotate-encryption-key, shared by both the
# API Portal and Platform API at-rest encryption keys (rotating either makes
# data encrypted under it permanently unreadable) - prompts at most once per
# run even though both keys are provisioned separately below.
$script:RotationConfirmed = $false
function Confirm-RotationOnce([string]$keyPath) {
    if ($script:RotationConfirmed) { return }
    $interactive = -not [Console]::IsInputRedirected
    if ($interactive -and [string]::IsNullOrEmpty($env:ADMIN_USERNAME) -and [string]::IsNullOrEmpty($env:ADMIN_PASSWORD)) {
        Write-Host ''
        Write-Host '  WARNING: --rotate-encryption-key will make any data encrypted'
        Write-Host '  under the current at-rest encryption key(s) (starting with'
        Write-Host "  $keyPath) permanently unreadable."
        $confirm = Read-Host "  Type 'rotate' to proceed"
        if ($confirm -ne 'rotate') { Invoke-Fail 'encryption key rotation not confirmed; aborting.' }
    } else {
        Write-Log '  - non-interactive/CI invocation: --rotate-encryption-key passed, treating that as confirmation'
    }
    $script:RotationConfirmed = $true
}

# Which pack's docker-compose.yaml this is - a single source of truth reused
# both to resolve the default COMPOSE_PROFILES value below and to print the
# right profile combinations at the end of this script. Prefers the value
# `make dist` baked into DEFAULT_PACK_NAME; falls back to this pack's own
# directory name, since both packs ship the same services and differ only in
# which one is their headline component. Mirrors detect_pack() in setup.sh.
function Get-Pack {
    if ($DEFAULT_PACK_NAME -ne '__DEFAULT_PACK_NAME__') { return $DEFAULT_PACK_NAME }
    $dir = Split-Path -Leaf $RootDir
    if ($dir -eq 'developer-portal') { return 'developer-portal' }
    if ($dir -eq 'ai-workspace') { return 'ai-workspace' }
    return 'unknown'
}

# --- Compose project name --------------------------------------------------------
# Compose accepts ^[a-z0-9][a-z0-9_-]*$ only - no dots, no uppercase. The comparisons
# below are case-SENSITIVE (-cmatch/-cnotmatch): PowerShell's -match is case-insensitive,
# which would accept "My_Workspace" and let Compose reject it later.
# $hint is an optional extra line naming where the rejected value came from.
function Test-ProjectName([string]$name, [string]$hint = '') {
    if ($name -cnotmatch '^[a-z0-9][a-z0-9_-]*$') {
        [Console]::Error.WriteLine("[setup] ERROR: invalid project name '$name' - must start with a lowercase letter or digit and contain only [a-z0-9_-]")
        if (-not [string]::IsNullOrEmpty($hint)) { [Console]::Error.WriteLine($hint) }
        exit 2
    }
}

# This pack's version: the value `make dist` baked in for a distribution zip, else the
# VERSION file a repo checkout has. Neither dist target copies VERSION into the zip,
# which is why the baked placeholder exists at all.
function Get-DistVersion {
    if ($DEFAULT_DIST_VERSION -ne '__DEFAULT_DIST_VERSION__') { return $DEFAULT_DIST_VERSION }
    $versionFile = Join-Path $RootDir 'VERSION'
    if (Test-Path -LiteralPath $versionFile) {
        return ([System.IO.File]::ReadAllText($versionFile)).Trim()
    }
    return ''
}

# <prefix>-<pack>-<sanitized version>-<6 hex>, e.g. wso2apip-ai-workspace-1-0-0-rc-a3f19c.
# The version is cosmetic: if it can't be read or sanitizes to nothing, fall back to
# <prefix>-<pack>-<6 hex> rather than failing setup. The suffix alone is what makes the
# name unique. Must produce the same name as setup.sh for the same inputs.
function New-ProjectName {
    $prefix = "$ProjectNamePrefix-$Pack"
    $version = Get-DistVersion
    if ($version.EndsWith('-SNAPSHOT')) {
        $version = $version.Substring(0, $version.Length - '-SNAPSHOT'.Length)
    }
    $version = $version.ToLowerInvariant() -replace '[^a-z0-9_-]', '-'
    $version = ($version -replace '-{2,}', '-').Trim('-')

    # openssl, not Get-Random: Get-Random is not a CSPRNG and would diverge from setup.sh.
    $suffix = (Invoke-OpenSslQuiet { & openssl rand -hex 3 } 'openssl failed to generate a project name suffix' | Out-String).Trim()

    $candidate = "$prefix-$suffix"
    if ($version -and "$prefix-$version-$suffix" -cmatch '^[a-z0-9][a-z0-9_-]*$') {
        $candidate = "$prefix-$version-$suffix"
    }
    return $candidate
}

# Reads the value the way compose itself does - it strips whitespace around an unquoted
# value, so "  name  " and "name" are the same pin and must validate the same way.
function Get-DotEnvProjectName {
    $line = @(Read-EnvLines $ComposeEnvFile |
        Where-Object { $_ -match '^\s*COMPOSE_PROJECT_NAME=' }) | Select-Object -Last 1
    if ($null -eq $line) { return '' }
    return (($line -replace '^[^=]*=', '') -replace '["'']', '').Trim()
}

# Key-scoped write: .env already holds COMPOSE_PROFILES (and possibly a dev's own keys),
# so never rewrite the whole file. Set-EnvVar cannot be used here - it replaces the value
# whenever --force is passed, which is exactly what must never happen to this key.
# Always UTF-8 without BOM and LF endings via Write-TextFileLf - a BOM makes Compose read
# the first key as "?COMPOSE_PROJECT_NAME" and silently ignore it.
function Write-DotEnvProjectName([string]$name) {
    $timestamp = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')

    if (-not (Test-Path -LiteralPath $ComposeEnvFile)) {
        $lines = @(
            "# Generated by scripts/setup.ps1 on $timestamp."
            '# Read automatically by "docker compose" from this directory. NOT passed to the containers.'
            '#'
            '# COMPOSE_PROJECT_NAME namespaces every container, network, and volume of this stack.'
            '# It is unique to THIS copy of the distribution, so another extraction of the same zip'
            '# elsewhere on this host cannot bind to this stack''s volumes.'
            '#'
            '# Do not change or delete this line after the first start: the running data lives in'
            '# volumes named <project>_platform-api-data etc., and a different name means the stack'
            '# starts with an empty database.'
            "COMPOSE_PROJECT_NAME=$name"
        )
        Write-TextFileLf $ComposeEnvFile (($lines -join "`n") + "`n")
        return
    }

    $existing = @(Read-EnvLines $ComposeEnvFile)
    $out = New-Object System.Collections.ArrayList
    if (@($existing | Where-Object { $_ -match '^\s*COMPOSE_PROJECT_NAME=' }).Count -gt 0) {
        $seen = $false
        foreach ($ln in $existing) {
            if ($ln -match '^\s*COMPOSE_PROJECT_NAME=') {
                if (-not $seen) { [void]$out.Add("COMPOSE_PROJECT_NAME=$name"); $seen = $true }
            } else {
                [void]$out.Add($ln)
            }
        }
    } else {
        foreach ($ln in $existing) { [void]$out.Add($ln) }
        # Drop the trailing empty element the final newline produces, so the appended
        # block doesn't accumulate blank lines on top of Set-EnvVar's own write.
        while ($out.Count -gt 0 -and $out[$out.Count - 1] -eq '') {
            $out.RemoveAt($out.Count - 1)
        }
        [void]$out.Add('')
        [void]$out.Add("# Added by scripts/setup.ps1 on $timestamp.")
        [void]$out.Add('# COMPOSE_PROJECT_NAME namespaces every container, network, and volume of this stack.')
        [void]$out.Add('# It is unique to THIS copy of the distribution, so another extraction of the same zip')
        [void]$out.Add('# elsewhere on this host cannot bind to this stack''s volumes.')
        [void]$out.Add('#')
        [void]$out.Add('# Do not change or delete this line after the first start: the running data lives in')
        [void]$out.Add('# volumes named <project>_platform-api-data etc., and a different name means the stack')
        [void]$out.Add('# starts with an empty database.')
        [void]$out.Add("COMPOSE_PROJECT_NAME=$name")
    }
    # Read-EnvLines yields a trailing empty element for the file's final newline;
    # writing it back verbatim would add one blank line to .env on every rerun.
    while ($out.Count -gt 0 -and $out[$out.Count - 1] -eq '') {
        $out.RemoveAt($out.Count - 1)
    }
    Write-TextFileLf $ComposeEnvFile (($out -join "`n") + "`n")
}

# Written once, on the first run, then never rotated - not on a rerun, not under --force,
# not under --rotate-encryption-key. Rotating it would orphan the live volumes and hand
# the operator an empty stack, which looks exactly like the bug this pinning exists to
# fix. An operator who wants a specific name (including the old extracted-folder name, to
# adopt a pre-.env install's volumes) sets COMPOSE_PROJECT_NAME in the environment for
# that first run.
function Set-ProjectName {
    # An empty "COMPOSE_PROJECT_NAME=" line counts as unset - Compose would fall back to
    # the directory name, which is the collision being fixed.
    $existing = Get-DotEnvProjectName

    if (-not [string]::IsNullOrEmpty($existing)) {
        # A hand-edited pin gets the same check as a generated one - otherwise setup reports
        # success on a stack docker compose then refuses to start.
        Test-ProjectName $existing '               from the COMPOSE_PROJECT_NAME line in .env'
        $script:ProjectName = $existing
        Write-Log "  - .env already pins COMPOSE_PROJECT_NAME=$existing - keeping it"
        # docker compose reads the environment ahead of .env, so a stale variable here
        # silently sends compose at a different project than this stack is pinned to.
        if ((-not [string]::IsNullOrEmpty($env:COMPOSE_PROJECT_NAME)) -and ($env:COMPOSE_PROJECT_NAME -cne $existing)) {
            Write-Log "    note: COMPOSE_PROJECT_NAME=$($env:COMPOSE_PROJECT_NAME) is set in this session and takes"
            Write-Log '          precedence over .env for every docker compose command run from it.'
        }
        return
    }

    if (-not [string]::IsNullOrEmpty($env:COMPOSE_PROJECT_NAME)) {
        $chosen = $env:COMPOSE_PROJECT_NAME; $source = 'COMPOSE_PROJECT_NAME from the environment'
    } else {
        $chosen = New-ProjectName; $source = 'generated'
    }

    Test-ProjectName $chosen
    Write-DotEnvProjectName $chosen
    $script:ProjectName = $chosen
    Write-Log "  - COMPOSE_PROJECT_NAME=$chosen written to .env ($source)"
}

Test-Prerequisites

$Pack = Get-Pack

Write-Log 'Setting default docker compose profiles ...'
# Resolution order: an explicit --profiles=<...> flag always wins; otherwise
# use the value `make dist` baked into DEFAULT_COMPOSE_PROFILES for a
# distribution zip; otherwise auto-detect from docker-compose.yaml, which is
# what happens when this script runs straight out of a repo checkout.
if (-not [string]::IsNullOrEmpty($ProfilesOverride)) {
    $ComposeProfilesValue = $ProfilesOverride
} elseif ($DEFAULT_COMPOSE_PROFILES -ne '__DEFAULT_COMPOSE_PROFILES__') {
    $ComposeProfilesValue = $DEFAULT_COMPOSE_PROFILES
} else {
    switch ($Pack) {
        'ai-workspace'     { $ComposeProfilesValue = 'ai-workspace,platform-api' }
        'developer-portal' { $ComposeProfilesValue = 'developer-portal,platform-api' }
        default            { $ComposeProfilesValue = '' }
    }
}

if (-not [string]::IsNullOrEmpty($ComposeProfilesValue)) {
    if (-not (Test-Path -LiteralPath $ComposeEnvFile)) { Write-TextFileLf $ComposeEnvFile '' }
    Set-EnvVar $ComposeEnvFile 'COMPOSE_PROFILES' $ComposeProfilesValue
} else {
    Write-Log '  - could not detect this pack''s default profiles; pass --profiles=<a,b,...> or always use --profile explicitly with docker compose'
}

# Same file, same section - .env is already created above and $Pack is resolved, and
# this one call site covers every path including the --certs-only exit below (which
# has already written .env by this point).
Write-Log 'Provisioning the docker compose project name ...'
Set-ProjectName

Write-Log 'Provisioning TLS certificate ...'
New-Item -ItemType Directory -Force -Path $CertsDir | Out-Null
# Shared by every service (mounted read-only into each at its own
# /etc/<service>/tls path) - SAN covers every hostname/container name either
# compose file uses, plus host.docker.internal for host-to-container access.
$CertPem = Join-Path $CertsDir 'cert.pem'
$KeyPem = Join-Path $CertsDir 'key.pem'
if (-not $Force -and (Test-Path -LiteralPath $CertPem) -and (Test-Path -LiteralPath $KeyPem)) {
    Write-Log "  - $CertsDir already has a certificate, leaving as-is"
} else {
    Invoke-OpenSslQuiet {
        & openssl req -x509 -newkey rsa:2048 -sha256 -days 3650 -nodes `
            -keyout $KeyPem -out $CertPem `
            -subj "/O=WSO2 API Platform/CN=localhost" `
            -addext "subjectAltName=DNS:localhost,DNS:*.localhost,DNS:platform-api,DNS:ai-workspace,DNS:api-portal,DNS:host.docker.internal,IP:127.0.0.1"
    } 'openssl failed to generate the TLS certificate' | Out-Null
    Set-OwnerOnlyAcl $KeyPem
    Write-Log "  - self-signed certificate generated at $CertsDir"
}

if ($CertsOnly) {
    exit 0
}

if (-not (Test-Path -LiteralPath $EnvFile)) { Write-TextFileLf $EnvFile '' }
# api-platform.env is read directly by the Docker CLI/daemon via env_file: -
# never bind-mounted into a container - so it only needs host-level protection.
Set-OwnerOnlyAcl $EnvFile

New-Item -ItemType Directory -Force -Path $KeysDir | Out-Null

Write-Log 'Provisioning API Portal encryption key and session secret ...'
# Written to files (not api-platform.env) and read by config.toml via
# {{ file "/etc/api-portal/keys/encryption.key" }} / {{ file ".../session-secret" }}
# - the same pattern as the Platform API's at-rest encryption key below, and
# for the same reason: a value that never appears in `docker inspect`, a
# process environment dump, or api-platform.env is materially harder to
# exfiltrate than an env var. resources\keys is mounted into the api-portal
# container at /etc/api-portal/keys (see docker-compose.yaml), which is on
# the API Portal's {{ file }} allowlist.
#
# api-portal-encryption.key encrypts subscription/webhook secrets at rest - so
# it is preserved across reruns and rotated ONLY via --rotate-encryption-key,
# never by the generic --force. api-portal-session-secret only signs session
# cookies - rotating it merely invalidates existing sessions, so it follows
# --force like the TLS cert/JWT keypair.
$ApiPortalEncKey = Join-Path $KeysDir 'api-portal-encryption.key'
$ApiPortalSessionSecret = Join-Path $KeysDir 'api-portal-session-secret'
if ((Test-Path -LiteralPath $ApiPortalEncKey) -and $RotateEncryptionKey) {
    Confirm-RotationOnce $ApiPortalEncKey
    New-HexSecretFile $ApiPortalEncKey
    Write-Log "  - API Portal encryption key ROTATED at $ApiPortalEncKey"
} elseif (Test-Path -LiteralPath $ApiPortalEncKey) {
    Write-Log "  - $ApiPortalEncKey already exists, leaving as-is (pass --rotate-encryption-key to replace it)"
} else {
    New-HexSecretFile $ApiPortalEncKey
    Write-Log "  - API Portal encryption key generated at $ApiPortalEncKey"
}
if (-not $Force -and (Test-Path -LiteralPath $ApiPortalSessionSecret)) {
    Write-Log "  - $ApiPortalSessionSecret already exists, leaving as-is"
} else {
    New-HexSecretFile $ApiPortalSessionSecret
    Write-Log "  - API Portal session secret generated at $ApiPortalSessionSecret"
}

Write-Log 'Provisioning Platform API JWT signing keypair (RS256) ...'
# Tokens are signed asymmetrically (RS256), not with a shared HMAC secret. The
# Platform API mints login tokens with the RSA private key and verifies every
# token with the matching public key. A PEM key is multi-line and does not
# survive an env file (one KEY=VALUE per line), so - like the TLS cert above -
# the keypair is written to files and read by config.toml via {{ file }}:
#   config.toml -> public_key/private_key = '{{ file "/etc/platform-api/keys/jwt_*.pem" }}'
# resources\keys is mounted into the platform-api container at
# /etc/platform-api/keys (see docker-compose.yaml), which is on the Platform
# API's {{ file }} allowlist.
$JwtPrivate = Join-Path $KeysDir 'jwt_private.pem'
$JwtPublic = Join-Path $KeysDir 'jwt_public.pem'
if (-not $Force -and (Test-Path -LiteralPath $JwtPrivate) -and (Test-Path -LiteralPath $JwtPublic)) {
    Write-Log "  - $KeysDir already has a JWT keypair, leaving as-is"
} else {
    # PKCS#8 private key + matching SPKI public key - the PEM encodings
    # golang-jwt's ParseRSAPrivateKeyFromPEM / ParseRSAPublicKeyFromPEM accept.
    Invoke-OpenSslQuiet {
        & openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out $JwtPrivate
    } 'openssl failed to generate the JWT private key' | Out-Null
    Invoke-OpenSslQuiet {
        & openssl rsa -in $JwtPrivate -pubout -out $JwtPublic
    } 'openssl failed to derive the JWT public key' | Out-Null
    Set-OwnerOnlyAcl $JwtPrivate
    Write-Log "  - RS256 JWT keypair generated at $KeysDir"
}

Write-Log 'Provisioning Platform API at-rest encryption key ...'
# Written to a file (not api-platform.env) and read by config.toml via
# {{ file "/etc/platform-api/keys/encryption.key" }} - the same mounted keys
# dir as the JWT keypair above. 32-byte key as 64 hex chars. Preserve an
# existing key across reruns - regenerating it makes previously-encrypted data
# permanently unreadable - so this key is rotated ONLY via the explicit,
# separate --rotate-encryption-key flag, never by the generic --force (which
# rotates the TLS cert / JWT keypair / admin credentials, none of which risk
# data loss the way rotating this key does).
$EncryptionKey = Join-Path $KeysDir 'encryption.key'
if ((Test-Path -LiteralPath $EncryptionKey) -and $RotateEncryptionKey) {
    Confirm-RotationOnce $EncryptionKey
    New-HexSecretFile $EncryptionKey
    Write-Log "  - at-rest encryption key ROTATED at $EncryptionKey"
} elseif (Test-Path -LiteralPath $EncryptionKey) {
    Write-Log "  - $EncryptionKey already exists, leaving as-is (pass --rotate-encryption-key to replace it)"
} else {
    New-HexSecretFile $EncryptionKey
    Write-Log "  - at-rest encryption key generated at $EncryptionKey"
}

Write-Log 'Provisioning Platform API admin credentials ...'
$CredentialsProvisioned = $false
$hasAdminUsername = Test-EnvKeySet $EnvFile 'APIP_CP_ADMIN_USERNAME'
$hasAdminHash = Test-EnvKeySet $EnvFile 'APIP_CP_ADMIN_PASSWORD_HASH'
$adminUsername = ''
$adminPassword = ''

if (-not $Force -and $hasAdminUsername -and -not $hasAdminHash) {
    Invoke-Fail 'api-platform.env has APIP_CP_ADMIN_USERNAME set but no APIP_CP_ADMIN_PASSWORD_HASH - a username paired with no hash (or a mismatched one) can never authenticate. Either delete APIP_CP_ADMIN_USERNAME from api-platform.env and re-run this script to regenerate both together, or re-run with --force to rotate both credentials at once.'
} elseif (-not $Force -and $hasAdminHash -and -not $hasAdminUsername) {
    Invoke-Fail 'api-platform.env has APIP_CP_ADMIN_PASSWORD_HASH set but no APIP_CP_ADMIN_USERNAME - a hash with no matching username can never authenticate. Either delete APIP_CP_ADMIN_PASSWORD_HASH from api-platform.env and re-run this script to regenerate both together, or re-run with --force to rotate both credentials at once.'
} elseif (-not $Force -and $hasAdminUsername -and $hasAdminHash) {
    Write-Log '  - APIP_CP_ADMIN_USERNAME already set in api-platform.env, leaving admin credentials as-is'
} else {
    $generated = (Invoke-OpenSslQuiet { & openssl rand -base64 24 } 'openssl failed to generate an admin password')
    $generated = ([string]$generated) -replace '[^A-Za-z0-9]', ''
    $generatedPassword = $generated.Substring(0, [Math]::Min(20, $generated.Length))
    if ([string]::IsNullOrEmpty($generatedPassword)) { Invoke-Fail 'failed to generate an admin password.' }

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

    $adminHash = Get-BcryptHash $adminPassword
    if ([string]::IsNullOrEmpty($adminHash)) {
        Invoke-Fail 'failed to hash the admin password (is docker able to pull httpd:2.4-alpine, or is htpasswd installed?).'
    }

    # Written raw, un-escaped - docker-compose.yaml's env_file: entries use
    # `format: raw`, which passes file content through byte-for-byte with no
    # ${VAR} interpolation, so a literal bcrypt hash ("$2y$12$...") survives
    # into the container as-is. Escaping "$" as "$$" here would corrupt it.
    Set-EnvVar $EnvFile 'APIP_CP_ADMIN_USERNAME' $adminUsername
    Set-EnvVar $EnvFile 'APIP_CP_ADMIN_PASSWORD_HASH' $adminHash

    $CredentialsProvisioned = $true
}

Write-Host ''
Write-Log 'Setup complete.'
Write-Host ''
if ($CredentialsProvisioned) {
    Write-Host '  ------------------------------------------------------------------'
    Write-Host "   Admin login:  $adminUsername / $adminPassword"
    Write-Host '   This password will not be shown again - copy it now.'
    Write-Host "   (It is stored, bcrypt-hashed, in api-platform.env's APIP_CP_ADMIN_PASSWORD_HASH)"
    Write-Host '  ------------------------------------------------------------------'
    Write-Host ''
}
Write-Host "  Compose project:  $ProjectName   (pinned in .env)"
Write-Host ''
Write-Host '  Next step - choose which components:'
Write-Host ''
# The first line always reflects $ComposeProfilesValue - the value actually
# just written to .env - rather than assuming $Pack's usual default. Those can
# differ: --profiles=<...> or a dist-baked DEFAULT_COMPOSE_PROFILES can set
# .env to something other than this pack's normal two-service combo.
if (-not [string]::IsNullOrEmpty($ComposeProfilesValue)) {
    Write-Host "    docker compose up -d                                                    # $ComposeProfilesValue (current .env default)"
} else {
    Write-Host '    docker compose up -d                                                    # no default set - pass --profile explicitly'
}
if ($Pack -eq 'developer-portal') {
    Write-Host '    docker compose --profile api-portal --profile ai-workspace --profile platform-api up -d  # API Portal + AI Workspace + Platform API'
    Write-Host '    docker compose --profile all up -d                                                              # AI Workspace + API Portal + Platform API'
    Write-Host '    docker compose --profile platform-api up -d                                                     # Platform API only'
} elseif ($Pack -eq 'ai-workspace') {
    Write-Host '    docker compose --profile ai-workspace --profile api-portal --profile platform-api up -d  # AI Workspace + API Portal + Platform API'
    Write-Host '    docker compose --profile all up -d                                                             # AI Workspace + API Portal + Platform API'
    Write-Host '    docker compose --profile platform-api up -d                                                    # Platform API only'
} else {
    Write-Host '    docker compose --profile <ai-workspace|api-portal|all|platform-api> up -d'
}
Write-Host ''
