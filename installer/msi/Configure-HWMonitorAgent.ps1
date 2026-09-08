[CmdletBinding()]
param(
	[AllowEmptyString()]
	[string]$HandshakeKey
)

$ErrorActionPreference = 'Stop'

$dataDir = Join-Path $env:ProgramData 'NITRINOnetControlManager'
$handshakeFile = Join-Path $dataDir 'handshake.key'
$apiPasswordFile = Join-Path $dataDir 'api.password'

New-Item -ItemType Directory -Path $dataDir -Force | Out-Null

# The MSI runs this script as LocalSystem. Protect the credentials from normal
# users, while allowing local administrators to rotate them when necessary.
$acl = New-Object System.Security.AccessControl.DirectorySecurity
$acl.SetAccessRuleProtection($true, $false)
$system = New-Object System.Security.Principal.SecurityIdentifier 'S-1-5-18'
$administrators = New-Object System.Security.Principal.SecurityIdentifier 'S-1-5-32-544'
$inheritance = [System.Security.AccessControl.InheritanceFlags]'ContainerInherit, ObjectInherit'
$propagation = [System.Security.AccessControl.PropagationFlags]::None
$rights = [System.Security.AccessControl.FileSystemRights]::FullControl
$acl.AddAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule($system, $rights, $inheritance, $propagation, 'Allow')))
$acl.AddAccessRule((New-Object System.Security.AccessControl.FileSystemAccessRule($administrators, $rights, $inheritance, $propagation, 'Allow')))
Set-Acl -LiteralPath $dataDir -AclObject $acl

# On upgrade an omitted property means “preserve the working panel key”.
if (-not [string]::IsNullOrWhiteSpace($HandshakeKey)) {
	[System.IO.File]::WriteAllText($handshakeFile, $HandshakeKey.Trim() + [Environment]::NewLine, [System.Text.UTF8Encoding]::new($false))
}

if (-not (Test-Path -LiteralPath $apiPasswordFile)) {
	$random = [System.Security.Cryptography.RandomNumberGenerator]::Create()
	$bytes = New-Object byte[] 32
	$random.GetBytes($bytes)
	[System.IO.File]::WriteAllText($apiPasswordFile, ([Convert]::ToHexString($bytes).ToLowerInvariant()) + [Environment]::NewLine, [System.Text.UTF8Encoding]::new($false))
}

