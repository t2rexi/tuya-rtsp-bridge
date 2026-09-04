#define MyAppName "Tuya RTSP Bridge"
#define MyAppVersion "1.2.5"
#define MyAppPublisher "Tuya RTSP Bridge contributors"
#define MyAppURL "https://github.com/DanEng1982/tuya-rtsp-bridge"
#define BundleDir "..\packaging\windows\staging"

[Setup]
AppId={{A7C3E91F-4B2D-4F11-9C08-7E2B91C4D001}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
DefaultDirName={localappdata}\Programs\TuyaRtspBridge
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
LicenseFile=..\LICENSE
OutputDir=output
OutputBaseFilename=TuyaRtspBridge-Setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
SetupIconFile=..\docs\images\app.ico
UninstallDisplayIcon={app}\app.ico
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
ShowLanguageDialog=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "german"; MessagesFile: "compiler:Languages\German.isl"
Name: "dutch"; MessagesFile: "compiler:Languages\Dutch.isl"
Name: "french"; MessagesFile: "compiler:Languages\French.isl"
Name: "spanish"; MessagesFile: "compiler:Languages\Spanish.isl"
Name: "brazilianportuguese"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"
Name: "italian"; MessagesFile: "compiler:Languages\Italian.isl"
Name: "polish"; MessagesFile: "compiler:Languages\Polish.isl"
Name: "czech"; MessagesFile: "compiler:Languages\Czech.isl"
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"
Name: "ukrainian"; MessagesFile: "compiler:Languages\Ukrainian.isl"
Name: "japanese"; MessagesFile: "compiler:Languages\Japanese.isl"
Name: "korean"; MessagesFile: "compiler:Languages\Korean.isl"
Name: "hebrew"; MessagesFile: "compiler:Languages\Hebrew.isl"
; Yiddish has no official Inno .isl — app UI still has ייִדיש.
; ChineseSimplified.isl is unofficial and not in every Inno install.
; App UI still has 中文 via the language menu after first start.

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
Source: "..\src\*"; DestDir: "{app}\src"; Flags: ignoreversion recursesubdirs; Excludes: "__pycache__\*,*.pyc"
Source: "..\web\*"; DestDir: "{app}\web"; Flags: ignoreversion recursesubdirs
Source: "..\vendor\tuya-ipc-terminal\*"; DestDir: "{app}\vendor\tuya-ipc-terminal"; Flags: ignoreversion recursesubdirs
Source: "{#BundleDir}\bin\*"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "{#BundleDir}\runtime\*"; DestDir: "{app}\runtime"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "{#BundleDir}\vlc\*"; DestDir: "{app}\vlc"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "..\launch.bat"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\launch-gui.vbs"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\launch-hidden.vbs"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\requirements.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\DEPENDENCIES.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.de.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.nl.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.fr.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.es.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.pt.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.it.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.pl.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.cs.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.ru.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.uk.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.id.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.zh.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.hi.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.ja.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.ko.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.he.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.yi.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\CHANGELOG.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\docs\legal.md"; DestDir: "{app}\docs"; Flags: ignoreversion
Source: "..\docs\rechtliches.md"; DestDir: "{app}\docs"; Flags: ignoreversion
Source: "..\SECURITY.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\NOTICE.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\CREDITS.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\CONTRIBUTING.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\docs\images\app.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\docs\images\logo.svg"; DestDir: "{app}\docs\images"; Flags: ignoreversion
Source: "..\docs\images\logo.png"; DestDir: "{app}\docs\images"; Flags: ignoreversion

[Icons]
Name: "{group}\Tuya RTSP Bridge"; Filename: "{app}\runtime\pythonw.exe"; Parameters: "-u ""{app}\src\gui.py"""; WorkingDir: "{app}"; IconFilename: "{app}\app.ico"
Name: "{autodesktop}\Tuya RTSP Bridge"; Filename: "{app}\runtime\pythonw.exe"; Parameters: "-u ""{app}\src\gui.py"""; WorkingDir: "{app}"; IconFilename: "{app}\app.ico"; Tasks: desktopicon
Name: "{group}\Tuya RTSP Bridge (console)"; Filename: "{app}\launch.bat"; WorkingDir: "{app}"; IconFilename: "{app}\app.ico"

[Run]
Filename: "{app}\runtime\pythonw.exe"; Parameters: "-u ""{app}\src\gui.py"""; WorkingDir: "{app}"; Description: "Start Tuya RTSP Bridge"; Flags: nowait postinstall skipifsilent skipifdoesntexist

[Code]
procedure CurStepChanged(CurStep: TSetupStep);
var
  LangFile: string;
begin
  if CurStep = ssPostInstall then
  begin
    LangFile := ExpandConstant('{userappdata}\TuyaRtspBridge\config.json');
    ForceDirectories(ExtractFilePath(LangFile));
    if ActiveLanguage = 'german' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "de"' + #13#10 + '}', False)
    else if ActiveLanguage = 'dutch' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "nl"' + #13#10 + '}', False)
    else if ActiveLanguage = 'french' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "fr"' + #13#10 + '}', False)
    else if ActiveLanguage = 'spanish' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "es"' + #13#10 + '}', False)
    else if ActiveLanguage = 'brazilianportuguese' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "pt"' + #13#10 + '}', False)
    else if ActiveLanguage = 'italian' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "it"' + #13#10 + '}', False)
    else if ActiveLanguage = 'polish' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "pl"' + #13#10 + '}', False)
    else if ActiveLanguage = 'czech' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "cs"' + #13#10 + '}', False)
    else if ActiveLanguage = 'russian' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "ru"' + #13#10 + '}', False)
    else if ActiveLanguage = 'ukrainian' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "uk"' + #13#10 + '}', False)
    else if ActiveLanguage = 'japanese' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "ja"' + #13#10 + '}', False)
    else if ActiveLanguage = 'korean' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "ko"' + #13#10 + '}', False)
    else if ActiveLanguage = 'hebrew' then
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "he"' + #13#10 + '}', False)
    else
      SaveStringToFile(LangFile, '{' + #13#10 + '  "lang": "en"' + #13#10 + '}', False);
  end;
end;
