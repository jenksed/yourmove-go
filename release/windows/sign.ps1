param([Parameter(Mandatory = $true)][string]$Artifact)
$ErrorActionPreference = 'Stop'
# Use an existing certificate/private key (including an attached hardware provider).
# Nothing is enrolled, purchased, imported, or self-signed by this script.
if (-not $env:YOURMOVE_WINDOWS_CERT_THUMBPRINT) { throw 'Set YOURMOVE_WINDOWS_CERT_THUMBPRINT to an existing trusted code-signing certificate.' }
if (-not $env:YOURMOVE_WINDOWS_TIMESTAMP_URL) { throw 'Set YOURMOVE_WINDOWS_TIMESTAMP_URL to an RFC 3161 timestamp service.' }
if (-not $env:YOURMOVE_WINDOWS_SIGNER_SUBJECT) { throw 'Set YOURMOVE_WINDOWS_SIGNER_SUBJECT to the approved certificate subject.' }
$tool = (Get-Command signtool.exe -ErrorAction Stop).Source
& $tool sign /sha1 $env:YOURMOVE_WINDOWS_CERT_THUMBPRINT /fd SHA256 /tr $env:YOURMOVE_WINDOWS_TIMESTAMP_URL /td SHA256 $Artifact
if ($LASTEXITCODE -ne 0) { throw "Signing failed: $Artifact" }
& $tool verify /pa /all /v $Artifact
if ($LASTEXITCODE -ne 0) { throw "Signature verification failed: $Artifact" }
$signature = Get-AuthenticodeSignature -LiteralPath $Artifact
if ($signature.Status -ne 'Valid' -or -not $signature.TimeStamperCertificate) { throw 'A valid, timestamped Authenticode signature is required.' }
if ($signature.SignerCertificate.Subject -ne $env:YOURMOVE_WINDOWS_SIGNER_SUBJECT) { throw 'Unexpected publisher certificate subject.' }
