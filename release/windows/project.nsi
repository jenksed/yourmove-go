; Based on Wails v3.1.0's project.nsi. Wails generates wails_tools.nsh.
; Stable identity is independent of publisher spelling and version.
Unicode true
!define WAILS_INSTALL_SCOPE "user"
!define REQUEST_EXECUTION_LEVEL "user"
!define UNINST_KEY_NAME "com.yourmove.go"
!include "wails_tools.nsh"
!include "WordFunc.nsh"
!include "MUI2.nsh"

VIProductVersion "${YOURMOVE_FILEVERSION}"
VIFileVersion "${YOURMOVE_FILEVERSION}"
VIAddVersionKey /LANG=1033 "CompanyName" "${INFO_COMPANYNAME}"
VIAddVersionKey /LANG=1033 "ProductName" "YourMove"
VIAddVersionKey /LANG=1033 "FileDescription" "YourMove Installer"
VIAddVersionKey /LANG=1033 "ProductVersion" "${INFO_PRODUCTVERSION}"
VIAddVersionKey /LANG=1033 "FileVersion" "${YOURMOVE_FILEVERSION}"
VIAddVersionKey /LANG=1033 "LegalCopyright" "${INFO_COPYRIGHT}"
ManifestDPIAware true
!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
!define MUI_ABORTWARNING
!define MUI_WELCOMEPAGE_TEXT "Setup installs YourMove for your Windows account.$\r$\n$\r$\nClose YourMove before upgrading. Saved work is retained.$\r$\n$\r$\nSetup downloads WebView2 if missing. Install Google Chrome separately to use employer-browser Fill."
!insertmacro MUI_PAGE_WELCOME
; A fixed per-user directory prevents installing over applicant documents.
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!define MUI_UNCONFIRMPAGE_TEXT_TOP "Uninstall YourMove? Your saved work and preferences will be retained."
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

!ifdef YOURMOVE_SIGN_SCRIPT
  ; The generated uninstaller must be signed before it is embedded.
  !uninstfinalize 'powershell.exe -NoProfile -NonInteractive -File "${YOURMOVE_SIGN_SCRIPT}" "%1"' = 0
!endif

Name "YourMove"
OutFile "${YOURMOVE_INSTALLER}"
InstallDir "$LOCALAPPDATA\Programs\YourMove"
ShowInstDetails show
ShowUninstDetails show

!macro refuse CODE MESSAGE
  IfSilent +2
    MessageBox MB_OK|MB_ICONSTOP "${MESSAGE}"
  SetErrorLevel ${CODE}
  Abort
!macroend

Function .onInit
  !insertmacro wails.checkArchitecture
  ${IfNot} ${AtLeastWin11}
    !insertmacro refuse 64 "YourMove requires Windows 11 x64."
  ${EndIf}
  SetShellVarContext current
  SetRegView 64
  ; Always use this path, including when /D is supplied. No data-directory override.
  StrCpy $INSTDIR "$LOCALAPPDATA\Programs\YourMove"
  ReadRegStr $0 HKCU "${UNINST_KEY}" "YourMoveFileVersion"
  ${If} $0 != ""
    ${VersionCompare} "$0" "${YOURMOVE_FILEVERSION}" $1
    ${If} $1 == 1
      !insertmacro refuse 66 "A newer YourMove version is installed. Downgrade refused."
    ${EndIf}
  ${EndIf}
FunctionEnd

; Check both documented Evergreen registration locations. A zero version is absent.
Function DetectWebView2
  StrCpy $0 ""
  ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${If} $0 == ""
  ${OrIf} $0 == "0.0.0.0"
    ReadRegStr $0 HKCU "Software\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}" "pv"
  ${EndIf}
FunctionEnd

; No automatic process termination or reboot replacement. Fail before modifying files.
!macro checkClosed
  IfFileExists "$INSTDIR\YourMove.exe" 0 closed
  System::Call 'kernel32::CreateFileW(w "$INSTDIR\YourMove.exe", i 0x40000000, i 0, p 0, i 3, i 0, p 0) p.r0'
  ${If} $0 == -1
    !insertmacro refuse 67 "Close YourMove and retry. Its installed executable cannot be replaced or removed."
  ${EndIf}
  System::Call 'kernel32::CloseHandle(p r0)'
  closed:
!macroend

Section "YourMove"
  !insertmacro wails.setShellContext
  SetRegView 64
  !insertmacro checkClosed
  Call DetectWebView2
  ${If} $0 == ""
  ${OrIf} $0 == "0.0.0.0"
    InitPluginsDir
    SetOutPath "$PLUGINSDIR"
    File "MicrosoftEdgeWebview2Setup.exe"
    ClearErrors
    ExecWait '"$PLUGINSDIR\MicrosoftEdgeWebview2Setup.exe" /silent /install' $1
    ${If} ${Errors}
      !insertmacro refuse 70 "Could not start Microsoft WebView2 setup. Check your connection and retry."
    ${EndIf}
    ${If} $1 != 0
      !insertmacro refuse 70 "Microsoft WebView2 setup failed. Check your connection and retry."
    ${EndIf}
    Call DetectWebView2
    ${If} $0 == ""
    ${OrIf} $0 == "0.0.0.0"
      !insertmacro refuse 70 "WebView2 is still unavailable. Install Microsoft WebView2 Evergreen Runtime and retry."
    ${EndIf}
  ${EndIf}
  SetOutPath "$INSTDIR"
  SetOverwrite on
  ClearErrors
  !insertmacro wails.files
  File /oname=LICENSE.txt "..\LICENSE.txt"
  File /oname=README.txt "..\README.txt"
  ${If} ${Errors}
    !insertmacro refuse 71 "Application files could not be installed. Close YourMove and retry setup."
  ${EndIf}
  ClearErrors
  !insertmacro wails.writeUninstaller
  WriteRegStr HKCU "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST_KEY}" "YourMoveFileVersion" "${YOURMOVE_FILEVERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "URLInfoAbout" "https://yourmove.run"
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST_KEY}" "NoRepair" 1
  CreateShortcut "$SMPROGRAMS\YourMove.lnk" "$INSTDIR\YourMove.exe" "" "$INSTDIR\YourMove.exe" 0
  ${If} ${Errors}
    !insertmacro refuse 71 "YourMove registration failed. Retry setup."
  ${EndIf}
SectionEnd

Section "uninstall"
  !insertmacro wails.setShellContext
  !insertmacro checkClosed
  ; Delete only installer-owned files. Never recurse into a user-data directory.
  ClearErrors
  Delete "$INSTDIR\YourMove.exe"
  Delete "$INSTDIR\LICENSE.txt"
  Delete "$INSTDIR\README.txt"
  ${If} ${Errors}
    !insertmacro refuse 71 "Some application files could not be removed. Close YourMove and retry."
  ${EndIf}
  Delete "$SMPROGRAMS\YourMove.lnk"
  !insertmacro wails.deleteUninstaller
  RMDir "$INSTDIR"
SectionEnd
