param([switch]$SkipDesktopBuild)
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
 if (!$SkipDesktopBuild) { & "$PSScriptRoot/build-desktop.ps1"; if($LASTEXITCODE){throw 'Desktop build failed'} }
 $compiler=Join-Path $env:WINDIR 'Microsoft.NET/Framework64/v4.0.30319/csc.exe'
 if(!(Test-Path $compiler)){throw 'Windows .NET Framework 4 compiler is required'}
 & $compiler /nologo /target:winexe /platform:x64 /optimize+ /define:UNINSTALL /reference:System.Windows.Forms.dll /reference:System.Drawing.dll /reference:System.IO.Compression.dll /win32icon:cmd\notsofast\app.ico /out:.tools\Uninstall.exe installer\Setup.cs
 if($LASTEXITCODE){throw 'Uninstaller build failed'}
 & $compiler /nologo /target:winexe /platform:x64 /optimize+ /reference:System.Windows.Forms.dll /reference:System.Drawing.dll /reference:System.IO.Compression.dll /win32icon:cmd\notsofast\app.ico /resource:dist\NotSoFast-Windows-x64.zip,payload.zip /resource:.tools\Uninstall.exe,uninstall.exe /out:dist\NotSoFast-Setup.exe installer\Setup.cs
 if($LASTEXITCODE){throw 'Installer build failed'}
 Get-FileHash dist\NotSoFast-Setup.exe -Algorithm SHA256
} finally {Pop-Location}

