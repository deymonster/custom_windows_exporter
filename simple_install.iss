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
Filename: "netsh"; Parameters: "advfirewall firewall add rule name=""NITRINOnet Control Manager API Port 9183"" protocol=TCP dir=in localport=9183 action=allow"; Flags: runhidden

[UninstallRun]
Filename: "sc"; Parameters: "stop NITRINOnetControlManager"; Flags: runhidden
Filename: "sc"; Parameters: "delete NITRINOnetControlManager"; Flags: runhidden
Filename: "netsh"; Parameters: "advfirewall firewall delete rule name=""NITRINOnet Control Manager Port 9182"""; Flags: runhidden
Filename: "netsh"; Parameters: "advfirewall firewall delete rule name=""NITRINOnet Control Manager API Port 9183"""; Flags: runhidden

[Registry]
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Services\EventLog\Application\NITRINOnetControlManager"; ValueType: string; ValueName: "EventMessageFile"; ValueData: "{app}\NITRINOnetControlManager.exe"; Flags: uninsdeletevalue
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Services\EventLog\Application\NITRINOnetControlManager"; ValueType: dword;  ValueName: "TypesSupported";    ValueData: "7"; Flags: uninsdeletevalue

[Code]
var
  CredentialsPage: TInputQueryWizardPage;
  HandshakeKey: string;
  ApiPassword: string;

procedure InitializeWizard();
begin
  // Страница ввода секретов
  CredentialsPage := CreateInputQueryPage(
    wpSelectTasks,
    'Параметры агента',
    'Handshake Key и API Password',
    'Введите общий Handshake Key и пароль API (будут записаны в ProgramData).'
  );
  CredentialsPage.Add('Handshake Key:', False);
  CredentialsPage.Add('API Password:', False);

  // Предзаполнение из командной строки для тихой установки
  HandshakeKey := ExpandConstant('{param:HANDSHAKE|}');
  ApiPassword := ExpandConstant('{param:API_PASSWORD|}');
  if HandshakeKey <> '' then CredentialsPage.Values[0] := HandshakeKey;
  if ApiPassword <> '' then CredentialsPage.Values[1] := ApiPassword;
end;

function NextButtonClick(CurPageID: Integer): Boolean;
begin
  Result := True;
  if CurPageID = CredentialsPage.ID then
  begin
    HandshakeKey := Trim(CredentialsPage.Values[0]);
    ApiPassword  := Trim(CredentialsPage.Values[1]);

    if WizardSilent then
    begin
      // В тихом режиме значения должны прийти параметрами /HANDSHAKE и /API_PASSWORD
      if (HandshakeKey = '') or (ApiPassword = '') then
      begin
        MsgBox('Для тихой установки задайте параметры /HANDSHAKE и /API_PASSWORD.', mbError, MB_OK);
        Result := False;
        exit;
      end;
    end
    else
    begin
      if HandshakeKey = '' then
      begin
        MsgBox('Введите Handshake Key.', mbError, MB_OK);
        Result := False;
        exit;
      end;
      if ApiPassword = '' then
      begin
        MsgBox('Введите API Password.', mbError, MB_OK);
        Result := False;
        exit;
      end;
    end;
  end;
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

    if ApiPassword <> '' then
      SaveStringToFile(PasswordPath, ApiPassword, False);
  end;
end;
