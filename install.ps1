# Install or update Death Counter. Run in PowerShell:
#   irm https://raw.githubusercontent.com/nguucode/death-counter/main/install.ps1 | iex
# First run asks for a folder; later runs update that folder in place.
# The app keeps its data in "DeathCounterData" inside the same folder.
# Messages are English on purpose: Windows PowerShell 5.1 can garble UTF-8 from irm.

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue' # the progress bar makes downloads very slow on 5.1
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$repo = 'nguucode/death-counter'
$key = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\DeathCounter'

$dir = (Get-ItemProperty $key -ErrorAction SilentlyContinue).InstallLocation
$update = [bool]$dir
if (-not $update) {
    $default = Join-Path $env:LOCALAPPDATA 'Programs\DeathCounter'
    $answer = Read-Host "Install folder (press Enter for $default)"
    $dir = if ($answer.Trim()) { [IO.Path]::GetFullPath($answer.Trim().Trim('"')) } else { $default }
    # Always use a folder of its own, so uninstall never touches other files.
    if ((Split-Path $dir -Leaf) -ne 'DeathCounter') { $dir = Join-Path $dir 'DeathCounter' }
}
$exe = Join-Path $dir 'DeathCounter.exe'

$release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
$asset = $release.assets | Where-Object name -eq 'DeathCounter.exe'
if (-not $asset) { throw "Release $($release.tag_name) has no DeathCounter.exe" }
$version = $release.tag_name.TrimStart('v')
Write-Host "Death Counter $version -> $dir"

# Close a running copy so the exe can be replaced; it saves its data on close.
Get-Process DeathCounter -ErrorAction SilentlyContinue | ForEach-Object {
    $_.CloseMainWindow() | Out-Null
    if (-not $_.WaitForExit(10000)) { $_ | Stop-Process -Force }
}

New-Item -ItemType Directory -Force $dir | Out-Null
$tmp = "$exe.download"
Invoke-WebRequest $asset.browser_download_url -OutFile $tmp -UseBasicParsing
Move-Item -Force $tmp $exe

$lnk = Join-Path ([Environment]::GetFolderPath('Programs')) 'Death Counter.lnk'
$shell = New-Object -ComObject WScript.Shell
$sc = $shell.CreateShortcut($lnk)
$sc.TargetPath = $exe
$sc.WorkingDirectory = $dir
$sc.Save()

# Shows the app in Windows "Apps & features" and remembers the folder for updates.
New-Item -Force $key | Out-Null
Set-ItemProperty $key DisplayName 'Death Counter'
Set-ItemProperty $key DisplayVersion $version
Set-ItemProperty $key Publisher 'nguucode'
Set-ItemProperty $key InstallLocation $dir
Set-ItemProperty $key DisplayIcon $exe
Set-ItemProperty $key UninstallString "`"$exe`" --uninstall"
Set-ItemProperty $key NoModify 1 -Type DWord
Set-ItemProperty $key NoRepair 1 -Type DWord

if ($update) { Write-Host "Updated to $version." } else { Write-Host "Installed $version. Start it from the Start menu: Death Counter." }
Start-Process $exe
