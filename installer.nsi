; Codex Go NSIS Installer
; Packages dist/desktop/release/Codex Go/ into a single Setup.exe
; Run from: dist/desktop/

!include "MUI2.nsh"

Name "Codex Go"
OutFile "Codex-Go-Setup-1.0.0.exe"
InstallDir "$PROGRAMFILES\Codex Go"
RequestExecutionLevel admin

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_LANGUAGE "English"

Section "Install"
  SetOutPath "$INSTDIR"
  File /r "release\Codex Go\*.*"

  ; Create shortcuts
  CreateDirectory "$SMPROGRAMS\Codex Go"
  CreateShortcut "$SMPROGRAMS\Codex Go\Codex Go.lnk" "$INSTDIR\Codex Go.exe"
  CreateShortcut "$DESKTOP\Codex Go.lnk" "$INSTDIR\Codex Go.exe"

  ; Write uninstaller
  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Registry for Add/Remove Programs
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CodexGo" \
    "DisplayName" "Codex Go"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CodexGo" \
    "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CodexGo" \
    "DisplayVersion" "1.0.0"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CodexGo" \
    "Publisher" "yeshenlougu"
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\*.*"
  RMDir /r "$INSTDIR"
  Delete "$SMPROGRAMS\Codex Go\Codex Go.lnk"
  RMDir "$SMPROGRAMS\Codex Go"
  Delete "$DESKTOP\Codex Go.lnk"
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\CodexGo"
SectionEnd
