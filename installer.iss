; Install script for NITRINOnet Control Manager
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

[Run]
Filename: "{app}\NITRINOnetControlManager.exe"; Parameters: "install"; Flags: runhidden
Filename: "netsh"; Parameters: "advfirewall firewall add rule name=""NITRINOnet Control Manager Port 9182"" protocol=TCP dir=in localport=9182 action=allow"; Flags: runhidden

[UninstallRun]
Filename: "{app}\remove_service.exe"; Parameters: "uninstall"; Flags: runhidden
Filename: "netsh"; Parameters: "advfirewall firewall delete rule name=""NITRINOnet Control Manager Port 9182"""; Flags: runhidden

[Code]
const
  EventSourceName = 'NITRINOnetControlManager';

procedure InitializeSetup();
begin
  if RegQueryStringValue(HKEY_LOCAL_MACHINE, 'SYSTEM\CurrentControlSet\Services\EventLog\Application\' + EventSourceName, 'EventMessageFile', ExpandConstant('')) = '' then
  begin
    RegWriteStringValue(HKEY_LOCAL_MACHINE, 'SYSTEM\CurrentControlSet\Services\EventLog\Application\' + EventSourceName, 'EventMessageFile', ExpandConstant('{app}\NITRINOnetControlManager.exe'));
    RegWriteDWordValue(HKEY_LOCAL_MACHINE, 'SYSTEM\CurrentControlSet\Services\EventLog\Application\' + EventSourceName, 'TypesSupported', 7);
  end;
end;

procedure DeinitializeSetup();
begin
  RegDeleteKeyIncludingSubkeys(HKEY_LOCAL_MACHINE, 'SYSTEM\CurrentControlSet\Services\EventLog\Application\' + EventSourceName);
end;