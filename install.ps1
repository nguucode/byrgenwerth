# Install or update Byrgenwerth. Run in PowerShell:
#   irm https://raw.githubusercontent.com/nguucode/byrgenwerth/main/install.ps1 | iex
# First run asks for a folder; later runs update that folder in place.
# The app keeps its data in "ByrgenwerthData" inside the same folder.
# An existing Death Counter install is renamed in place; its data comes along.
# Messages are English on purpose: Windows PowerShell 5.1 can garble UTF-8 from irm.

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue' # the progress bar makes downloads very slow on 5.1
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = 'nguucode/byrgenwerth'
# Before touching anything, so a release without the exe leaves an install as it was.
$release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
$asset = $release.assets | Where-Object name -eq 'Byrgenwerth.exe'
if (-not $asset) { throw "Release $($release.tag_name) has no Byrgenwerth.exe" }
$version = $release.tag_name.TrimStart('v')

$key = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\Byrgenwerth'
$oldKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\DeathCounter'

function Stop-App($name) {
    # Close a running copy so the exe can be replaced; it saves its data on close.
    Get-Process $name -ErrorAction SilentlyContinue | ForEach-Object {
        $_.CloseMainWindow() | Out-Null
        if (-not $_.WaitForExit(10000)) { $_ | Stop-Process -Force }
    }
}

$dir = (Get-ItemProperty $key -ErrorAction SilentlyContinue).InstallLocation
$oldDir = (Get-ItemProperty $oldKey -ErrorAction SilentlyContinue).InstallLocation
if (-not $dir -and $oldDir) {
    Stop-App 'DeathCounter'
    $dir = $oldDir
    if ((Test-Path $oldDir) -and (Split-Path $oldDir -Leaf) -eq 'DeathCounter') {
        $dir = Join-Path (Split-Path $oldDir -Parent) 'Byrgenwerth'
        Rename-Item $oldDir $dir
    }
    # Remember the renamed folder now, so a failed download can simply be run again.
    New-Item -Force $key | Out-Null
    Set-ItemProperty $key InstallLocation $dir
    Remove-Item $oldKey
}
$update = [bool]$dir
if (-not $update) {
    $default = Join-Path $env:LOCALAPPDATA 'Programs\Byrgenwerth'
    $answer = Read-Host "Install folder (press Enter for $default)"
    $dir = if ($answer.Trim()) { [IO.Path]::GetFullPath($answer.Trim().Trim('"')) } else { $default }
    # Always use a folder of its own, so uninstall never touches other files.
    if ((Split-Path $dir -Leaf) -ne 'Byrgenwerth') { $dir = Join-Path $dir 'Byrgenwerth' }
}
$exe = Join-Path $dir 'Byrgenwerth.exe'

Write-Host "Byrgenwerth $version -> $dir"

Stop-App 'Byrgenwerth'

New-Item -ItemType Directory -Force $dir | Out-Null
$tmp = "$exe.download"
# The API link: github.com/.../releases/download can answer 504 while the API link still works.
Invoke-WebRequest $asset.url -Headers @{ Accept = 'application/octet-stream' } -OutFile $tmp -UseBasicParsing
Move-Item -Force $tmp $exe
# Left behind by Death Counter, removed only once the new exe is in place.
Remove-Item (Join-Path $dir 'DeathCounter.exe') -ErrorAction SilentlyContinue
Remove-Item (Join-Path ([Environment]::GetFolderPath('Programs')) 'Death Counter.lnk') -ErrorAction SilentlyContinue

$lnk = Join-Path ([Environment]::GetFolderPath('Programs')) 'Byrgenwerth.lnk'
$shell = New-Object -ComObject WScript.Shell
$sc = $shell.CreateShortcut($lnk)
$sc.TargetPath = $exe
$sc.WorkingDirectory = $dir
$sc.Save()

# Shows the app in Windows "Apps & features" and remembers the folder for updates.
New-Item -Force $key | Out-Null
Set-ItemProperty $key DisplayName 'Byrgenwerth'
Set-ItemProperty $key DisplayVersion $version
Set-ItemProperty $key Publisher 'nguucode'
Set-ItemProperty $key InstallLocation $dir
Set-ItemProperty $key DisplayIcon $exe
Set-ItemProperty $key UninstallString "`"$exe`" --uninstall"
Set-ItemProperty $key NoModify 1 -Type DWord
Set-ItemProperty $key NoRepair 1 -Type DWord

if ($update) { Write-Host "Updated to $version." } else { Write-Host "Installed $version. Start it from the Start menu: Byrgenwerth." }
Start-Process $exe
