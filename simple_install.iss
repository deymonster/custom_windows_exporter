; Install script for NITRINOnet Control Manager (Шаг 1: Простой установщик файлов)
[Setup]
AppName=NITRINOnet Control Manager
AppVersion=1.0.1
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

[Icons]
Name: "{group}\NITRINOnet Control Manager"; Filename: "{app}\NITRINOnetControlManager.exe"; IconFilename: "{app}\app_ico.ico"
Name: "{group}\Uninstall NITRINOnet Control Manager"; Filename: "{uninstallexe}"


[Run]
; Создаем службу и добавляем автозапуск
Filename: "sc"; Parameters: "create NITRINOnetControlManager binPath= ""{app}\NITRINOnetControlManager.exe"" DisplayName= ""NITRINOnet Control Manager"" start= auto"; Flags: runhidden
; Запуск службы после создания
Filename: "sc"; Parameters: "start NITRINOnetControlManager"; Flags: runhidden
; Настройка правила брандмауэра
Filename: "netsh"; Parameters: "advfirewall firewall add rule name=""NITRINOnet Control Manager Port 9182"" protocol=TCP dir=in localport=9182 action=allow"; Flags: runhidden


[UninstallRun]
; Удаляем службу
Filename: "sc"; Parameters: "stop NITRINOnetControlManager"; Flags: runhidden
Filename: "sc"; Parameters: "delete NITRINOnetControlManager"; Flags: runhidden
; Удаление правила брандмауэра
Filename: "netsh"; Parameters: "advfirewall firewall delete rule name=""NITRINOnet Control Manager Port 9182"""; Flags: runhidden

[Registry]
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Services\EventLog\Application\NITRINOnetControlManager"; ValueType: string; ValueName: "EventMessageFile"; ValueData: "{app}\NITRINOnetControlManager.exe"; Flags: uninsdeletevalue
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Services\EventLog\Application\NITRINOnetControlManager"; ValueType: dword; ValueName: "TypesSupported"; ValueData: "7"; Flags: uninsdeletevalue