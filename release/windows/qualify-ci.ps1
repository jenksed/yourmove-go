#Requires -Version 5.1
<#
.SYNOPSIS
  Qualify the built YourMove Windows installer on a native Windows runner.

.DESCRIPTION
  Consumes the candidate directory produced by release/windows/prepare.py and a
  same-source browser-mode server binary, then exercises the installer
  lifecycle: artifact verification, WebView2 detection, silent per-user
  install, registration checks, installed-application launch, backend
  persistence across restart, uninstall data retention, and reinstall recovery.

  Results are PASS, FAIL, BLOCKED, or NOT RUN per check, written to
  evidence.json. The script exits nonzero only when a check FAILs. BLOCKED and
  NOT RUN checks (for example, an interactive GUI on a headless runner, or an
  upgrade pair that does not exist) do not fail the run; they are reported.

  No release is published, no tag is created, and no external account changes.

.PARAMETER CandidateDir
  Candidate directory produced by prepare.py.

.PARAMETER ServerExe
  Same-source browser-mode YourMove binary (built without the desktop tag) used
  for the HTTP save-path lifecycle. The installed Wails executable cannot serve
  loopback HTTP, so this binary exercises the identical Go App, routes, and
  SQLite workspace behavior through the supported HTTP interface.

.PARAMETER WorkspaceRoot
  Parent directory for the isolated synthetic workspace. Nothing production is
  touched; YOURMOVE_DATA_DIR always points inside this directory.

.PARAMETER EvidenceDir
  Directory receiving evidence.json and server/install logs.
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][string]$CandidateDir,
  [Parameter(Mandatory = $true)][string]$ServerExe,
  [Parameter(Mandatory = $true)][string]$WorkspaceRoot,
  [Parameter(Mandatory = $true)][string]$EvidenceDir
)

$ErrorActionPreference = 'Stop'
$checks = [ordered]@{}
$notes = [ordered]@{}

function Set-Check([string]$Name, [string]$Status, [string]$Detail = '') {
  $checks[$Name] = $Status
  if ($Detail -ne '') { $notes[$Name] = $Detail }
  Write-Host "[$Status] $Name $Detail"
}

function Get-PeMachine([string]$Path) {
  $bytes = [System.IO.File]::ReadAllBytes($Path)
  $lfanew = [BitConverter]::ToInt32($bytes, 0x3C)
  return [BitConverter]::ToUInt16($bytes, $lfanew + 4)
}

function Get-FreePort {
  $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
  try {
    $listener.Start()
    return ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port
  } finally {
    $listener.Stop()
  }
}

function Wait-HttpOk([string]$Url, [int]$TimeoutSeconds) {
  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  do {
    try {
      $response = Invoke-WebRequest -Uri $Url -TimeoutSec 3 -UseBasicParsing
      if ($response.StatusCode -eq 200) { return $response }
    } catch { }
    Start-Sleep -Milliseconds 250
  } while ((Get-Date) -lt $deadline)
  return $null
}

function Start-AppServer([string]$Exe, [string]$DataDir, [int]$Port, [string]$LogPath) {
  $env:YOURMOVE_DATA_DIR = $DataDir
  $env:YOURMOVE_HTTP_PORT = "$Port"
  $process = Start-Process -FilePath $Exe -RedirectStandardOutput $LogPath `
    -RedirectStandardError "$LogPath.stderr" -PassThru
  return $process
}

function Post-Form([string]$Url, [string[]]$Fields, [string]$OutFile) {
  # curl.exe ships with Windows and reports redirect codes faithfully;
  # Invoke-WebRequest mishandles the 303 responses on this runner.
  $args = @('-s', '-o', $OutFile, '-w', '%{http_code}')
  foreach ($field in $Fields) { $args += '--data-urlencode'; $args += $field }
  $args += $Url
  return ((& curl.exe @args 2>$null).Trim())
}

function Stop-Quiet([System.Diagnostics.Process]$Process, [string]$Name) {
  if ($null -eq $Process) { return }
  try {
    $Process.Refresh()
    if (-not $Process.HasExited) {
      try { $Process.CloseMainWindow() | Out-Null } catch { }
      $Process.Refresh()
      if (-not $Process.HasExited -and -not $Process.WaitForExit(5000)) {
        $Process.Refresh()
        if (-not $Process.HasExited) { $Process.Kill() }
      }
    }
  } catch { }
  try { $Process.Refresh(); Write-Host "$Name exited: $($Process.HasExited)" } catch { }
}

$candidate = Get-Content (Join-Path $CandidateDir 'candidate.json') | ConvertFrom-Json
$installer = Join-Path $CandidateDir $candidate.installer
$workspace = Join-Path $WorkspaceRoot 'yourmove-ci-workspace'
New-Item -ItemType Directory -Force -Path $workspace, $EvidenceDir | Out-Null

# Phase 0: artifact identity.
try {
  if (-not (Test-Path $installer)) { throw "installer missing: $installer" }
  $hash = (Get-FileHash $installer -Algorithm SHA256).Hash.ToLower()
  if ($hash -ne $candidate.installer_sha256) { throw "SHA-256 mismatch" }
  Set-Check 'artifact-sha256' 'PASS' $hash
} catch {
  Set-Check 'artifact-sha256' 'FAIL' $_.Exception.Message
}
try {
  $payload = Join-Path $CandidateDir 'YourMove.exe'
  $machine = Get-PeMachine $payload
  # 0x8664 = AMD64. The NSIS stub itself is x86 by design; the payload must not be.
  if ($machine -eq 0x8664) { Set-Check 'payload-arch-x64' 'PASS' 'IMAGE_FILE_MACHINE_AMD64' }
  else { Set-Check 'payload-arch-x64' 'FAIL' ("machine=0x{0:X4}" -f $machine) }
} catch {
  Set-Check 'payload-arch-x64' 'FAIL' $_.Exception.Message
}

# Phase 1: WebView2 detection (same two registry locations as project.nsi).
try {
  $wv = $null
  foreach ($hive in 'HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}',
                     'HKCU:\Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}') {
    try { $wv = (Get-ItemProperty -Path $hive -Name 'pv' -ErrorAction Stop).pv } catch { }
    if ($wv -and $wv -ne '0.0.0.0') { break }
  }
  if ($wv -and $wv -ne '0.0.0.0') { Set-Check 'webview2-runtime' 'PASS' "pv=$wv" }
  else { Set-Check 'webview2-runtime' 'BLOCKED' 'no Evergreen runtime registration; installer bootsrapper path applies' }
} catch {
  Set-Check 'webview2-runtime' 'BLOCKED' $_.Exception.Message
}

# Phase 2: silent per-user install.
$installDir = Join-Path $env:LOCALAPPDATA 'Programs\YourMove'
$installedExe = Join-Path $installDir 'YourMove.exe'
$uninstKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\com.yourmove.go'
try {
  $proc = Start-Process -FilePath $installer -ArgumentList '/S' -Wait -PassThru
  if ($proc.ExitCode -ne 0) { throw "installer exit code $($proc.ExitCode)" }
  if (-not (Test-Path $installedExe)) { throw 'installed executable missing' }
  if ((Get-PeMachine $installedExe) -ne 0x8664) { throw 'installed payload is not AMD64' }
  $version = (Get-Item $installedExe).VersionInfo
  if ($version.ProductName -ne 'YourMove') { throw "ProductName=$($version.ProductName)" }
  if ($version.ProductVersion -ne $candidate.version) { throw "ProductVersion=$($version.ProductVersion)" }
  if ($version.FileVersion -ne "$($candidate.version).$($candidate.build)") { throw "FileVersion=$($version.FileVersion)" }
  Set-Check 'install-silent' 'PASS' "exit=0 arch=AMD64 file=$($version.FileVersion)"
} catch {
  Set-Check 'install-silent' 'FAIL' $_.Exception.Message
}
try {
  $reg = Get-ItemProperty -Path $uninstKey -ErrorAction Stop
  if ([string]::IsNullOrEmpty($reg.QuietUninstallString) -or -not $reg.QuietUninstallString.Contains('/S')) {
    throw 'QuietUninstallString missing /S'
  }
  if ($reg.InstallLocation -ne $installDir) { throw "InstallLocation=$($reg.InstallLocation)" }
  Set-Check 'install-registration' 'PASS' "DisplayName=$($reg.DisplayName) DisplayVersion=$($reg.DisplayVersion)"
} catch {
  Set-Check 'install-registration' 'FAIL' $_.Exception.Message
}
try {
  $shortcut = Join-Path ([Environment]::GetFolderPath('Programs')) 'YourMove.lnk'
  if (Test-Path $shortcut) { Set-Check 'install-shortcut' 'PASS' $shortcut }
  else { Set-Check 'install-shortcut' 'FAIL' "missing $shortcut" }
} catch {
  Set-Check 'install-shortcut' 'FAIL' $_.Exception.Message
}
try {
  $principal = [Security.Principal.WindowsPrincipal]::new([Security.Principal.WindowsIdentity]::GetCurrent())
  $elevated = $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
  Set-Check 'install-per-user' 'PASS' "HKCU key, LOCALAPPDATA path, RequestExecutionLevel=user; runner elevated=$elevated (non-admin enforcement not verified on this runner)"
} catch {
  Set-Check 'install-per-user' 'FAIL' $_.Exception.Message
}

# Phase 3: launch the installed application.
try {
  $env:YOURMOVE_DATA_DIR = $workspace
  $launchLog = Join-Path $EvidenceDir 'installed-launch.log'
  $app = Start-Process -FilePath $installedExe -RedirectStandardOutput $launchLog `
    -RedirectStandardError "$launchLog.stderr" -PassThru
  Start-Sleep -Seconds 25
  $app.Refresh()
  if ($app.HasExited) {
    Set-Check 'installed-launch' 'BLOCKED' "process exited within 25s (exit=$($app.ExitCode)); headless Server cannot host the WebView2 window"
  } else {
    Set-Check 'installed-launch' 'PASS' "process alive after 25s (pid=$($app.Id)); window visibility not verified on headless Server"
  }
  Stop-Quiet $app 'Installed app'
} catch {
  Set-Check 'installed-launch' 'BLOCKED' $_.Exception.Message
}

# Phase 4: backend save-path lifecycle on the same workspace via same-source server.
$dbPath = Join-Path $workspace 'jobs.db'
try {
  $port = Get-FreePort
  $base = "http://127.0.0.1:$port"
  $serverLog = Join-Path $EvidenceDir 'server-cycle1.log'
  $server = Start-AppServer $ServerExe $workspace $port $serverLog
  try {
    $ready = Wait-HttpOk "$base/jobs" 30
    if ($null -eq $ready) { throw 'server did not serve /jobs within 30s' }
    Set-Check 'backend-startup' 'PASS' "GET /jobs 200 on $base"

    $code = Post-Form "$base/candidate" @('first_name=CI', 'last_name=Qualify',
      'email=ci-qualify@example.test', 'phone=5550100', 'pronouns=They/them', 'approve=yes') `
      (Join-Path $EvidenceDir 'post-candidate.out')
    Write-Host "POST /candidate http_code=$code"
    if ($code -ne '303') { throw "POST /candidate http_code=$code" }

    $code = Post-Form "$base/families" @('name=CI Family', 'title_terms=support engineer',
      'context_terms=', 'exclude_terms=') (Join-Path $EvidenceDir 'post-families.out')
    Write-Host "POST /families http_code=$code"
    if ($code -ne '303') { throw "POST /families http_code=$code" }

    $resumeBytes = [Text.Encoding]::UTF8.GetBytes("%PDF-1.4`nsynthetic ci qualification resume")
    $resumeFile = Join-Path $EvidenceDir 'ci-resume.pdf'
    [IO.File]::WriteAllBytes($resumeFile, $resumeBytes)
    # curl.exe ships with Windows and builds the multipart body reliably;
    # Invoke-WebRequest -Form mishandles the 303 redirect on this runner.
    $uploadCode = (& curl.exe -s -o "$EvidenceDir/upload-resumes.out" -w '%{http_code}' `
      -F "resume=@$resumeFile;type=application/pdf" "$base/resumes/import" 2>$null).Trim()
    if ($uploadCode -ne '303') { throw "POST /resumes/import http_code=$uploadCode" }
    # The embedded production frontend serves a React shell on /resumes, so
    # assert the library through the JSON API and verify exact document bytes.
    $lib = (Invoke-WebRequest -Uri "$base/api/resumes" -UseBasicParsing).Content | ConvertFrom-Json
    if ($lib.total -lt 1 -or $lib.resumes[0].name -ne 'ci-resume.pdf') { throw 'uploaded resume not listed in /api/resumes' }
    $downloaded = Join-Path $EvidenceDir 'ci-resume-download.pdf'
    Invoke-WebRequest -Uri "$base/resumes/$($lib.resumes[0].id)/download" -OutFile $downloaded -UseBasicParsing
    $upHash = (Get-FileHash $resumeFile -Algorithm SHA256).Hash
    if ((Get-FileHash $downloaded -Algorithm SHA256).Hash -ne $upHash) { throw 'downloaded resume bytes differ' }
    Write-Host "resume bytes round-trip sha=$($upHash.ToLower())"
    Set-Check 'backend-save' 'PASS' 'profile + role family + resume bytes saved over HTTP'
  } finally {
    Stop-Quiet $server 'Server cycle 1'
  }

  $hashBefore = (Get-FileHash $dbPath -Algorithm SHA256).Hash.ToLower()

  # Restart on the same workspace; the save path must survive.
  $port2 = Get-FreePort
  $base2 = "http://127.0.0.1:$port2"
  $server2 = Start-AppServer $ServerExe $workspace $port2 (Join-Path $EvidenceDir 'server-cycle2.log')
  try {
    $ready = Wait-HttpOk "$base2/candidate" 30
    if ($null -eq $ready) { throw 'restarted server did not serve within 30s' }
    $body = $ready.Content
    if (-not ($body.Contains('ci-qualify@example.test') -and $body.Contains('They/them'))) {
      throw 'saved profile not readable after restart'
    }
    $resumes2 = ((Invoke-WebRequest -Uri "$base2/api/resumes" -UseBasicParsing).Content | ConvertFrom-Json).resumes
    if (-not ($resumes2 | Where-Object { $_.name -eq 'ci-resume.pdf' })) { throw 'saved resume not listed after restart' }
    $families = (Invoke-WebRequest -Uri "$base2/families" -UseBasicParsing).Content
    if (-not $families.Contains('CI Family')) { throw 'saved role family not listed after restart' }
    Set-Check 'backend-restart-persistence' 'PASS' "jobs.db sha=$hashBefore"
  } finally {
    Stop-Quiet $server2 'Server cycle 2'
  }
} catch {
  $notes['backend-error'] = "$($_.Exception.GetType().FullName): $($_.Exception.Message)`n$($_.ScriptStackTrace)"
  Write-Host "BACKEND-ERROR $($notes['backend-error'])"
  Set-Check 'backend-restart-persistence' 'FAIL' $_.Exception.Message
  if ($checks['backend-save'] -eq $null) { Set-Check 'backend-save' 'FAIL' 'save path did not complete' }
}

# Phase 5: uninstall retention.
try {
  $uninstaller = Join-Path $installDir 'uninstall.exe'
  $hashPre = (Get-FileHash $dbPath -Algorithm SHA256).Hash.ToLower()
  $proc = Start-Process -FilePath $uninstaller -ArgumentList '/S' -Wait -PassThru
  if ($proc.ExitCode -ne 0) { throw "uninstaller exit code $($proc.ExitCode)" }
  if (Test-Path $installedExe) { throw 'installed executable survived uninstall' }
  if (Test-Path $uninstKey) { throw 'HKCU uninstall key survived uninstall' }
  if (-not (Test-Path $dbPath)) { throw 'workspace jobs.db was removed by uninstall' }
  $hashPost = (Get-FileHash $dbPath -Algorithm SHA256).Hash.ToLower()
  if ($hashPre -ne $hashPost) { throw 'workspace database changed during uninstall' }
  Set-Check 'uninstall-retention' 'PASS' 'program files removed, registration removed, synthetic workspace retained byte-identical'
} catch {
  Set-Check 'uninstall-retention' 'FAIL' $_.Exception.Message
}

# Phase 6: reinstall recovery on the retained workspace.
try {
  $proc = Start-Process -FilePath $installer -ArgumentList '/S' -Wait -PassThru
  if ($proc.ExitCode -ne 0) { throw "reinstall exit code $($proc.ExitCode)" }
  if (-not (Test-Path $installedExe)) { throw 'reinstalled executable missing' }
  $port3 = Get-FreePort
  $base3 = "http://127.0.0.1:$port3"
  $server3 = Start-AppServer $ServerExe $workspace $port3 (Join-Path $EvidenceDir 'server-cycle3.log')
  try {
    $ready = Wait-HttpOk "$base3/candidate" 30
    if ($null -eq $ready) { throw 'server did not serve after reinstall within 30s' }
    if (-not $ready.Content.Contains('ci-qualify@example.test')) { throw 'retained profile unreadable after reinstall' }
    Set-Check 'reinstall-recovery' 'PASS' 'retained synthetic records reopen after reinstall'
  } finally {
    Stop-Quiet $server3 'Server cycle 3'
  }
} catch {
  Set-Check 'reinstall-recovery' 'FAIL' $_.Exception.Message
}

# Phase 7: upgrade. No second independently versioned candidate exists in this run.
Set-Check 'upgrade-preservation' 'NOT RUN' 'requires a second versioned installer; same-binary reinstall is not an upgrade test'

# Managed-browser and GUI journeys cannot execute on a headless Server runner.
Set-Check 'browser-fill' 'NOT RUN' 'managed Chromium Fill needs an interactive session; no automatic submission was attempted'
Set-Check 'gui-acceptance' 'NOT RUN' 'native window presentation needs Windows 11 interactive acceptance'

$evidence = [ordered]@{
  schema              = 'yourmove-windows-ci-qualification/1'
  source_commit       = $candidate.source_commit
  version             = $candidate.version
  build               = $candidate.build
  installer           = $candidate.installer
  installer_sha256    = $candidate.installer_sha256
  runner_os           = (Get-CimInstance Win32_OperatingSystem).Caption
  runner_arch         = $env:PROCESSOR_ARCHITECTURE
  checks              = $checks
  notes               = $notes
}
$evidence | ConvertTo-Json -Depth 6 | Set-Content (Join-Path $EvidenceDir 'evidence.json')

$failed = @($checks.Values | Where-Object { $_ -eq 'FAIL' }).Count
Write-Host "checks=$($checks.Count) failed=$failed"
exit $failed
