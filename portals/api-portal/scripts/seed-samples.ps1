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
# Windows counterpart of scripts/seed-samples.sh: same behavior, same
# generated output. Deploys the bundled sample APIs and MCP servers into a
# running Developer Portal, entirely through its public REST API — no
# in-process seeding logic ships in the application itself.
#
# Prerequisites: ..\scripts\setup.ps1 has been run and `docker compose up` is
# running. Unlike seed-samples.sh, no external jq/zip is required: JSON is
# parsed with ConvertFrom-Json and the docs archive is built with
# System.IO.Compression (both built into PowerShell 5.1+). curl.exe IS still
# required — PowerShell's own `curl`/`wget` aliases point at Invoke-WebRequest,
# which this script deliberately avoids so multipart file uploads (-F) work
# exactly as they do in the bash version.
#
# Usage (from the project root, or the standalone distribution zip — same
# layout in both):
#   powershell -ExecutionPolicy Bypass -File .\scripts\seed-samples.ps1
#   pwsh -File ./scripts/seed-samples.ps1                # PowerShell 7+ is also fine
#
# ADMIN_USERNAME / ADMIN_PASSWORD environment variables skip the interactive
# credential prompt (used by CI). API_PORTAL_URL / PLATFORM_API_URL override
# the default local URLs.
#
# Safe to re-run: entries that already exist (matched by name + version) are
# skipped, not duplicated.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Write-Log($msg) { Write-Host "[seed-samples] $msg" }
function Invoke-Fail($msg) {
    [Console]::Error.WriteLine("[seed-samples] ERROR: $msg")
    exit 1
}

function Test-CommandExists($name) {
    return [bool](Get-Command $name -ErrorAction SilentlyContinue)
}

$ThisDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# Same layout-detection approach as setup.ps1 — this script is copied verbatim
# into the distribution zip's scripts\ directory (see the Makefile's dist
# target), and both layouts put it one level below docker-compose.yaml.
if (Test-Path -LiteralPath (Join-Path $ThisDir '..\docker-compose.yaml')) {
    $RootDir = (Resolve-Path (Join-Path $ThisDir '..')).Path
} elseif (Test-Path -LiteralPath (Join-Path $ThisDir 'docker-compose.yaml')) {
    $RootDir = $ThisDir
} else {
    Invoke-Fail 'could not find docker-compose.yaml next to this script or its parent directory. Run this as .\scripts\seed-samples.ps1 from the project root or the distribution zip.'
}

# The distribution zip ships samples under resources\samples\; the source
# repo keeps them at samples\ (see the Makefile's dist target for the copy).
if (Test-Path -LiteralPath (Join-Path $RootDir 'resources\samples')) {
    $SamplesDir = Join-Path $RootDir 'resources\samples'
} elseif (Test-Path -LiteralPath (Join-Path $RootDir 'samples')) {
    $SamplesDir = Join-Path $RootDir 'samples'
} else {
    Invoke-Fail "no samples directory found (looked for resources\samples and samples next to $RootDir)."
}

$ApiPortalUrl = if ($env:API_PORTAL_URL) { $env:API_PORTAL_URL } else { 'https://localhost:9543' }
$PlatformApiUrl = if ($env:PLATFORM_API_URL) { $env:PLATFORM_API_URL } else { 'https://localhost:9243' }

# Colors only when writing to an interactive terminal (respects the NO_COLOR
# convention: https://no-color.org/) — a piped/CI log gets plain ASCII symbols
# instead. Only the status symbol and the trailing detail are colored (bash
# colors more of each line, but PowerShell's Write-Host colors a whole call,
# not a substring, so segmenting further isn't worth the extra complexity).
$UseColor = [string]::IsNullOrEmpty($env:NO_COLOR) -and (-not [Console]::IsOutputRedirected)
if ($UseColor) {
    $SymOk = [char]0x2713; $SymFail = [char]0x2717; $SymSkip = [char]0x2022
} else {
    $SymOk = 'OK'; $SymFail = 'FAIL'; $SymSkip = '-'
}

function Write-SampleLine([string]$Symbol, [string]$SymbolColor, [string]$Name, [string]$Detail, [string]$DetailColor = 'DarkGray') {
    Write-Host -NoNewline '  '
    if ($UseColor) { Write-Host -NoNewline $Symbol -ForegroundColor $SymbolColor } else { Write-Host -NoNewline $Symbol }
    Write-Host -NoNewline (' {0,-28} ' -f $Name)
    if ($Detail) {
        if ($UseColor) { Write-Host $Detail -ForegroundColor $DetailColor } else { Write-Host $Detail }
    } else {
        Write-Host ''
    }
}

if (-not (Test-CommandExists 'curl.exe')) {
    Invoke-Fail 'curl.exe is required but not found on PATH. Windows 10 (1803+) and Windows 11 ship it; Git for Windows and Docker Desktop also provide it.'
}

# curl runs with -s throughout, which normally means no stderr output at all —
# but this still mirrors setup.ps1's Invoke-OpenSslQuiet pattern rather than
# relying on that alone: under $ErrorActionPreference = 'Stop', any
# native-command stderr write becomes a terminating NativeCommandError that
# `2>$null` alone does not reliably suppress on Windows PowerShell 5.1.
function Invoke-CurlQuiet([scriptblock]$Script) {
    $prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        return & $Script 2>$null
    } finally {
        $ErrorActionPreference = $prev
    }
}

$AdminUsername = $env:ADMIN_USERNAME
if ([string]::IsNullOrEmpty($AdminUsername) -and (-not [Console]::IsInputRedirected)) {
    $AdminUsername = Read-Host 'API Portal admin username'
}
if ([string]::IsNullOrEmpty($AdminUsername)) { Invoke-Fail 'an admin username is required (set ADMIN_USERNAME or run interactively).' }

$AdminPassword = $env:ADMIN_PASSWORD
if ([string]::IsNullOrEmpty($AdminPassword) -and (-not [Console]::IsInputRedirected)) {
    $secure = Read-Host 'API Portal admin password' -AsSecureString
    $bstr = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try {
        $AdminPassword = [System.Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    } finally {
        [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    }
}
if ([string]::IsNullOrEmpty($AdminPassword)) { Invoke-Fail 'an admin password is required (set ADMIN_PASSWORD or run interactively).' }

Write-Log "Logging in to Platform API at $PlatformApiUrl ..."
# Percent-encode both values — a raw '&'/'='/'+'/'%' in either would otherwise
# split or corrupt the application/x-www-form-urlencoded body.
$encodedUsername = [uri]::EscapeDataString($AdminUsername)
$encodedPassword = [uri]::EscapeDataString($AdminPassword)
$loginBody = "username=$encodedUsername&password=$encodedPassword"
$loginResponse = Invoke-CurlQuiet { & curl.exe -sk -X POST "$PlatformApiUrl/api/portal/v0.9/auth/login" -d $loginBody }
# PowerShell splits a native command's multi-line stdout into a string ARRAY,
# one element per line — casting that straight to [string] joins elements
# with a space (via $OFS), not a newline, silently corrupting JSON that
# happens to span more than one line. @(...) -join "`n" reassembles it
# correctly whether curl returned one line (a plain string) or several.
$loginJson = (@($loginResponse) -join "`n")
$Token = $null
try { $Token = ($loginJson | ConvertFrom-Json).token } catch { $Token = $null }
if ([string]::IsNullOrEmpty($Token)) {
    Invoke-Fail "failed to obtain a token — check the credentials and that Platform API is reachable at $PlatformApiUrl."
}
$AuthHeader = "Authorization: Bearer $Token"

$StartTime = Get-Date
$ApiCreated = 0; $ApiSkipped = 0; $ApiFailed = 0
$McpCreated = 0; $McpSkipped = 0; $McpFailed = 0

function Add-Counter([string]$Endpoint, [string]$Field) {
    if ($Endpoint -eq 'mcp-servers') {
        switch ($Field) {
            'CREATED' { $script:McpCreated++ }
            'SKIPPED' { $script:McpSkipped++ }
            'FAILED'  { $script:McpFailed++ }
        }
    } else {
        switch ($Field) {
            'CREATED' { $script:ApiCreated++ }
            'SKIPPED' { $script:ApiSkipped++ }
            'FAILED'  { $script:ApiFailed++ }
        }
    }
}

# Builds a ZIP whose entries are rooted at "<sampleName>/docs/..." rather than
# "docs/..." — the server unwraps a single top-level directory entry looking
# for web/ or docs/ inside it; zipping docs/ bare as that one entry gets
# unwrapped too, so it then looks for (and fails to find) docs/docs. Built
# directly via System.IO.Compression rather than staging a copy, so no
# external `zip` tool is needed on Windows.
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

function New-DocsZip([string]$SampleDir, [string]$DestZip) {
    $sampleName = Split-Path -Leaf $SampleDir
    $docsDir = Join-Path $SampleDir 'docs'
    if (Test-Path -LiteralPath $DestZip) { Remove-Item -LiteralPath $DestZip -Force }
    $zip = [System.IO.Compression.ZipFile]::Open($DestZip, [System.IO.Compression.ZipArchiveMode]::Create)
    try {
        Get-ChildItem -LiteralPath $docsDir -Recurse -File | ForEach-Object {
            $relative = $_.FullName.Substring($docsDir.Length + 1) -replace '\\', '/'
            $entryName = "$sampleName/docs/$relative"
            [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile($zip, $_.FullName, $entryName) | Out-Null
        }
    } finally {
        $zip.Dispose()
    }
}

# Uploads sample_dir\docs\ as the content ZIP for an already-created API/MCP
# server. Returns @{ Result = <fragment or ''>; Failed = <bool> } so
# Invoke-SeedEntry can fold the fragment into that sample's single summary
# line and tally the outcome into the right counter.
function Invoke-SeedDocs([string]$SampleDir, [string]$ResourcePath) {
    $docsDir = Join-Path $SampleDir 'docs'
    if (-not (Test-Path -LiteralPath $docsDir -PathType Container)) {
        return @{ Result = ''; Failed = $false }
    }

    $tmpZip = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName() -replace '\.\w+$', '.zip')
    New-DocsZip $SampleDir $tmpZip

    $rawCode = Invoke-CurlQuiet {
        & curl.exe -sk -o NUL -w '%{http_code}' -X POST `
            "$ApiPortalUrl$ResourcePath/assets" `
            -H $AuthHeader `
            -F "content=@$tmpZip;type=application/zip"
    }
    $httpCode = ((@($rawCode) -join '')).Trim()
    Remove-Item -LiteralPath $tmpZip -Force -ErrorAction SilentlyContinue

    if ($httpCode -match '^2\d\d$') {
        return @{ Result = "docs $SymOk"; Failed = $false }
    } else {
        return @{ Result = "docs $SymFail ($httpCode)"; Failed = $true }
    }
}

# Creates one API or MCP server entry from a sample directory (api.yaml +
# optional definition.* + optional docs\), via the given collection endpoint.
# Prints exactly one summary line per sample and tallies the outcome into the
# *Created/*Skipped/*Failed counters (bucketed by $Endpoint).
function Invoke-SeedEntry([string]$SampleDir, [string]$Endpoint) {
    $name = Split-Path -Leaf $SampleDir
    $apiYaml = Join-Path $SampleDir 'api.yaml'

    if (-not (Test-Path -LiteralPath $apiYaml)) {
        Write-SampleLine $SymSkip 'Yellow' $name '(no api.yaml, skipped)'
        Add-Counter $Endpoint 'SKIPPED'
        return
    }

    # $ErrorActionPreference is 'Stop' script-wide, so a terminating error from
    # any call below (e.g. New-DocsZip hitting a locked/permission-denied file
    # under docs\) would otherwise propagate out of this function and abort
    # the whole ForEach-Object loop — every remaining sample would then be
    # silently skipped instead of just this one being tallied as FAILED.
    try {
        $definition = Get-ChildItem -LiteralPath $SampleDir -Filter 'definition.*' -File -ErrorAction SilentlyContinue |
            Sort-Object Name | Select-Object -First 1

        $curlArgs = @('-sk', '-X', 'POST', "$ApiPortalUrl/api/v0.9/$Endpoint",
            '-H', $AuthHeader,
            '-F', "metadata=@$apiYaml;type=application/yaml")
        if ($definition) {
            $curlArgs += @('-F', "definition=@$($definition.FullName);type=application/octet-stream")
        }

        $response = Invoke-CurlQuiet { & curl.exe @curlArgs -w "`n%{http_code}" }
        # See the Invoke-SeedDocs comment above — @(...) forces a possibly-scalar
        # result into an array first, so a genuinely multi-line array response
        # isn't collapsed into one space-joined string before splitting.
        $lines = @($response)
        $httpCode = ([string]$lines[-1]).Trim()
        $body = if ($lines.Length -gt 1) { ($lines[0..($lines.Length - 2)] -join "`n") } else { '' }

        if ($httpCode -match '^2\d\d$') {
            $id = $null
            try { $id = ($body | ConvertFrom-Json).id } catch { $id = $null }
            $docs = @{ Result = ''; Failed = $false }
            if ($id) { $docs = Invoke-SeedDocs $SampleDir "/api/v0.9/$Endpoint/$id" }

            if ($docs.Failed) {
                # Entry itself was created, but its docs upload failed — surface
                # this as a failure (red symbol, FAILED tally) rather than a
                # clean success, so the closing summary's failed count isn't
                # silently undercounted.
                Write-SampleLine $SymFail 'Red' $name "(id: $id, $($docs.Result))" 'Red'
                Add-Counter $Endpoint 'FAILED'
            } elseif ($docs.Result) {
                Write-SampleLine $SymOk 'Green' $name "(id: $id, $($docs.Result))"
                Add-Counter $Endpoint 'CREATED'
            } else {
                Write-SampleLine $SymOk 'Green' $name "(id: $id)"
                Add-Counter $Endpoint 'CREATED'
            }
        } elseif ($httpCode -eq '409') {
            Write-SampleLine $SymSkip 'Yellow' $name '(already exists)'
            Add-Counter $Endpoint 'SKIPPED'
        } else {
            $shortErr = $null
            try {
                $parsed = $body | ConvertFrom-Json
                $shortErr = if ($parsed.error) { $parsed.error } else { $parsed.message }
            } catch { $shortErr = $null }
            if ([string]::IsNullOrEmpty($shortErr)) { $shortErr = $body }
            Write-SampleLine $SymFail 'Red' $name "($httpCode`: $shortErr)" 'Red'
            Add-Counter $Endpoint 'FAILED'
        }
    } catch {
        Write-SampleLine $SymFail 'Red' $name "(unexpected error: $($_.Exception.Message))" 'Red'
        Add-Counter $Endpoint 'FAILED'
    }
}

$apisDir = Join-Path $SamplesDir 'apis'
if (Test-Path -LiteralPath $apisDir -PathType Container) {
    Write-Host ''
    Write-Host 'Seeding APIs'
    Get-ChildItem -LiteralPath $apisDir -Directory | ForEach-Object { Invoke-SeedEntry $_.FullName 'apis' }
}

$mcpsDir = Join-Path $SamplesDir 'mcps'
if (Test-Path -LiteralPath $mcpsDir -PathType Container) {
    Write-Host ''
    Write-Host 'Seeding MCP servers'
    Get-ChildItem -LiteralPath $mcpsDir -Directory | ForEach-Object { Invoke-SeedEntry $_.FullName 'mcp-servers' }
}

$TotalCreated = $ApiCreated + $McpCreated
$TotalSkipped = $ApiSkipped + $McpSkipped
$TotalFailed = $ApiFailed + $McpFailed
$Elapsed = [int]((Get-Date) - $StartTime).TotalSeconds

Write-Host ''
$statusColor = if ($TotalFailed -gt 0) { 'Red' } else { 'Green' }
$apiWord = if ($ApiCreated -eq 1) { 'API' } else { 'APIs' }
$mcpWord = if ($McpCreated -eq 1) { 'MCP server' } else { 'MCP servers' }
$summary = "$TotalCreated seeded ($ApiCreated $apiWord, $McpCreated $mcpWord), $TotalSkipped skipped, $TotalFailed failed in ${Elapsed}s"
if ($UseColor) {
    Write-Host -NoNewline 'Done' -ForegroundColor $statusColor
    Write-Host " — $summary"
} else {
    Write-Host "Done — $summary"
}

# CI pins credentials via ADMIN_USERNAME/ADMIN_PASSWORD specifically so this
# script can run unattended — a failing seed must not be reported as success.
if ($TotalFailed -gt 0) { exit 1 }
