; Install script for NITRINOnet Control Manager (Шаг 1: Простой установщик файлов)
; Stored with Windows (CRLF) line endings for Inno Setup compatibility.

; Параметризуем версию установщика через препроцессор
#ifndef MyAppVersion
  #define MyAppVersion "dev"
#endif
#ifndef MyAppFileVersion
  #define MyAppFileVersion "0.0.0.0"
#endif

[Setup]
AppName=NITRINOnet Control Manager
AppVersion={#MyAppVersion}
VersionInfoVersion={#MyAppFileVersion}
VersionInfoTextVersion={#MyAppVersion}
DefaultDirName={pf}\NITRINOnet Control Manager
DefaultGroupName=NITRINOnet Control Manager
OutputDir=.
OutputBaseFilename=NITRINOnetControlManagerSetup
Compression=lzma
SolidCompression=yes
PrivilegesRequired=admin
UninstallDisplayIcon={app}\NITRINOnetControlManager.exe

SetupIconFile=app_ico.ico

[Files]
Source: "NITRINOnetControlManager.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "remove_service.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "app_ico.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "configs\certs\cert.pem"; DestDir: "{commonappdata}\NITRINOnetControlManager\certs"; Flags: ignoreversion
Source: "configs\certs\key.pem"; DestDir: "{commonappdata}\NITRINOnetControlManager\certs"; Flags: ignoreversion

[Icons]
Name: "{group}\NITRINOnet Control Manager"; Filename: "{app}\NITRINOnetControlManager.exe"; IconFilename: "{app}\app_ico.ico"
Name: "{group}\Uninstall NITRINOnet Control Manager"; Filename: "{uninstallexe}"

[Run]
Filename: "sc"; Parameters: "create NITRINOnetControlManager binPath= ""{app}\NITRINOnetControlManager.exe"" DisplayName= ""NITRINOnet Control Manager"" start= auto"; Flags: runhidden
Filename: "sc"; Parameters: "start NITRINOnetControlManager"; Flags: runhidden
Filename: "netsh"; Parameters: "advfirewall firewall add rule name=""NITRINOnet Control Manager Port 9182"" protocol=TCP dir=in localport=9182 action=allow"; Flags: runhidden

[UninstallRun]
Filename: "sc"; Parameters: "stop NITRINOnetControlManager"; Flags: runhidden
Filename: "sc"; Parameters: "delete NITRINOnetControlManager"; Flags: runhidden
Filename: "netsh"; Parameters: "advfirewall firewall delete rule name=""NITRINOnet Control Manager Port 9182"""; Flags: runhidden

[Registry]
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Services\EventLog\Application\NITRINOnetControlManager"; ValueType: string; ValueName: "EventMessageFile"; ValueData: "{app}\NITRINOnetControlManager.exe"; Flags: uninsdeletevalue
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Services\EventLog\Application\NITRINOnetControlManager"; ValueType: dword;  ValueName: "TypesSupported";    ValueData: "7"; Flags: uninsdeletevalue

[Code]
var
  CredentialsPage: TInputQueryWizardPage;
  HandshakeKey: string;

procedure InitializeWizard();
begin
  // The panel handshake key is the only value an administrator provides.
  // The local API password is generated on this computer during installation.
  CredentialsPage := CreateInputQueryPage(
    wpSelectTasks,
    'Параметры агента',
    'Подключение к панели',
    'Введите Handshake Key, выданный администратором панели.'
  );
  CredentialsPage.Add('Handshake Key:', False);

  // Pre-fill from the command line for unattended installation.
  HandshakeKey := ExpandConstant('{param:HANDSHAKE|}');
  if HandshakeKey <> '' then CredentialsPage.Values[0] := HandshakeKey;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID = CredentialsPage.ID then
  begin
    HandshakeKey := Trim(CredentialsPage.Values[0]);

    if HandshakeKey = '' then
    begin
      if WizardSilent then
        MsgBox('Для тихой установки задайте параметр /HANDSHAKE.', mbError, MB_OK)
      else
        MsgBox('Введите Handshake Key.', mbError, MB_OK);
      Result := False;
      exit;
    end;
  end;
end;

function GenerateApiPassword(const PasswordPath: string): Boolean;
var
  ExitCode: Integer;
  PowerShell: string;
begin
  PowerShell := ExpandConstant('{sys}\WindowsPowerShell\v1.0\powershell.exe');
  Result := Exec(
    PowerShell,
    '-NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "$bytes=New-Object byte[] 32;[System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes);$value=([System.BitConverter]::ToString($bytes)).Replace(''-'', '''').ToLowerInvariant();[System.IO.File]::WriteAllText(''' + PasswordPath + ''',$value+[Environment]::NewLine,(New-Object System.Text.UTF8Encoding($false)))"',
    '', SW_HIDE, ewWaitUntilTerminated, ExitCode) and (ExitCode = 0);
end;

procedure RestrictSecretFile(const SecretPath: string);
var
  ExitCode: Integer;
begin
  if not Exec(ExpandConstant('{sys}\icacls.exe'),
    '"' + SecretPath + '" /inheritance:r /grant:r "*S-1-5-18:(F)" "*S-1-5-32-544:(F)"',
    '', SW_HIDE, ewWaitUntilTerminated, ExitCode) or (ExitCode <> 0) then
    RaiseException('Не удалось ограничить доступ к локальному API-password.');
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  BaseDir, HandshakePath, PasswordPath: string;
begin
  if CurStep = ssInstall then
  begin
    BaseDir := ExpandConstant('{commonappdata}\NITRINOnetControlManager');
    HandshakePath := BaseDir + '\handshake.key';
    PasswordPath  := BaseDir + '\api.password';

    if not DirExists(BaseDir) then
      ForceDirectories(BaseDir);

    if HandshakeKey <> '' then
      SaveStringToFile(HandshakePath, HandshakeKey, False);

    // Keep the existing secret during upgrades; create it only for a new agent.
    if not FileExists(PasswordPath) then
    begin
      if not GenerateApiPassword(PasswordPath) then
        RaiseException('Не удалось создать локальный API-password.');
    end;
    RestrictSecretFile(PasswordPath);
  end;
end;
