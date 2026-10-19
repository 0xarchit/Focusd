!ifndef VERSION
  !define VERSION "0.0.0"
!endif

!include "MUI2.nsh"
!include "WinMessages.nsh"

; Everything focusd touches is per-user: the install dir under %APPDATA%, HKCU
; registry, and the user PATH. No elevation is needed or wanted.
RequestExecutionLevel user


Name "Focusd"
OutFile "focusd_setup.exe"
InstallDir "$APPDATA\focusd"

; Registry key for uninstall info
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Focusd"

; Interface Settings
!define MUI_ABORTWARNING
!define MUI_ICON "assets\focusd.ico"
!define MUI_UNICON "assets\focusd.ico"

; Pages
!insertmacro MUI_PAGE_WELCOME
; No install-directory page: everything is pinned to $APPDATA\focusd. A custom
; directory produced duplicate binaries and a mismatched PATH entry (issue #18).
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_WELCOME
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_UNPAGE_FINISH

; Languages
!insertmacro MUI_LANGUAGE "English"

Section "Install"
  SetOutPath "$INSTDIR"
  
  ; Stop the daemon and any running TUI before overwriting the binaries.
  DetailPrint "Stopping existing daemon if active..."
  IfFileExists "$INSTDIR\focusd.exe" 0 +3
    ExecWait '"$INSTDIR\focusd.exe" stop'
    Sleep 1000
  ExecWait 'taskkill.exe /F /IM focusd_daemon.exe'
  ExecWait 'taskkill.exe /F /IM focusd.exe'
  Sleep 500

  File "focusd.exe"
  File "focusd_daemon.exe"

  ; Delete legacy Startup shortcut if it exists
  Delete "$SMSTARTUP\Focus Daemon.lnk"

  ; Add to user Autostart registry run key
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "focusd" '"$INSTDIR\focusd_daemon.exe"'

  ; Add to user PATH environment variable.
  ; PATH is owned solely by "focusd.exe path enable" so there is one
  ; implementation. Two independent implementations caused issue #18, where
  ; install and uninstall overwrote or cleared the user's PATH.
  DetailPrint "Adding focusd to your user PATH..."
  IfFileExists "$INSTDIR\focusd.exe" 0 skip_path_enable
    ExecWait '"$INSTDIR\focusd.exe" path enable' $0
    StrCmp $0 "0" +2
      DetailPrint "WARNING: could not add focusd to PATH (exit code $0). Run 'focusd path enable' manually."
  skip_path_enable:

  ; Write registry keys for Windows Add/Remove Programs (uninstaller)
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayName" "Focusd Screen Time Tracker"
  WriteRegStr HKCU "${UNINST_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST_KEY}" "Publisher" "0xarchit"
  WriteRegStr HKCU "${UNINST_KEY}" "DisplayIcon" '"$INSTDIR\focusd.exe"'

  WriteUninstaller "$INSTDIR\uninstall.exe"

  ; Start the daemon immediately in a detached process context
  DetailPrint "Starting Focusd daemon..."
  ExecShell "open" "$INSTDIR\focusd_daemon.exe" "" SW_HIDE
SectionEnd

Section "Uninstall"
  ; Stop the daemon process gracefully
  DetailPrint "Stopping daemon process..."
  IfFileExists "$INSTDIR\focusd.exe" 0 +3
    ExecWait '"$INSTDIR\focusd.exe" stop'
    Sleep 1000

  ; Force kill if still running to release file locks
  ExecWait 'taskkill.exe /F /IM focusd_daemon.exe'
  ExecWait 'taskkill.exe /F /IM focusd.exe'
  Sleep 500

  ; Remove from user PATH while the exe still exists. focusd.exe owns the
  ; logic; see the install section.
  IfFileExists "$INSTDIR\focusd.exe" 0 skip_path_disable
    ExecWait '"$INSTDIR\focusd.exe" path disable' $0
    StrCmp $0 "0" +2
      DetailPrint "WARNING: could not remove focusd from PATH (exit code $0). Run 'focusd path disable' manually."
  skip_path_disable:

  Delete "$INSTDIR\focusd.exe"
  Delete "$INSTDIR\focusd_daemon.exe"
  Delete "$INSTDIR\focusd_start.vbs"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "focusd"
  Delete "$SMSTARTUP\Focus Daemon.lnk"
  Delete "$INSTDIR\uninstall.exe"

  ; Remove registry uninstall keys
  DeleteRegKey HKCU "${UNINST_KEY}"

  ; $INSTDIR also holds focusd.db and config.json, which is the user's tracking
  ; history. It is deliberately not removed, so uninstall and reinstall keep the
  ; data. The directory is only removable once it is empty anyway.
  RMDir "$INSTDIR"
SectionEnd

