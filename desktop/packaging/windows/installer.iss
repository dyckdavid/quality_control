; Inno Setup script for the Quality Control desktop app.
; Build: iscc /DAppVersion=0.1.0 /DSourceExe=..\..\target\release\qc-desktop.exe /DOutputDir=..\..\target\dist installer.iss
; To sign the setup and uninstaller, also pass /DSign "/Sqcsign=<signtool command ending in $f>".

#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif
#ifndef SourceExe
  #define SourceExe "..\..\target\release\qc-desktop.exe"
#endif
#ifndef OutputDir
  #define OutputDir "..\..\target\dist"
#endif

[Setup]
AppId={{6F2B8C1E-4A3D-4F6B-9C2E-7D1A5B8E3F40}
AppName=Quality Control
AppVersion={#AppVersion}
AppPublisher=David Dyck
DefaultDirName={autopf}\Quality Control
DefaultGroupName=Quality Control
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutputDir}
OutputBaseFilename=QualityControl-{#AppVersion}-windows-setup
SetupIconFile=..\..\assets\icon.ico
UninstallDisplayIcon={app}\QualityControl.exe
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
#ifdef Sign
SignTool=qcsign
SignedUninstaller=yes
#endif

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
Source: "{#SourceExe}"; DestDir: "{app}"; DestName: "QualityControl.exe"; Flags: ignoreversion

[Icons]
Name: "{group}\Quality Control"; Filename: "{app}\QualityControl.exe"
Name: "{autodesktop}\Quality Control"; Filename: "{app}\QualityControl.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\QualityControl.exe"; Description: "{cm:LaunchProgram,Quality Control}"; Flags: nowait postinstall skipifsilent
