!ifndef VERSION
  !define VERSION "0.0.0"
!endif

!ifndef BINARY_DIR
  !define BINARY_DIR "."
!endif
!ifndef OUTPUT_FILE
  !define OUTPUT_FILE "focusd_setup.exe"
!endif

!include "MUI2.nsh"
!include "WinMessages.nsh"

; Everything focusd touches is per-user: the install dir under %APPDATA%, HKCU
; registry, and the user PATH. No elevation is needed or wanted.
RequestExecutionLevel user


Name "Focusd"
OutFile "${OUTPUT_FILE}"
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

; Bundle the new control protocol even when replacing an older CLI. This helper
; is named differently so scoped focusd.exe exit checks cannot count themselves.
!macro PrepareFocusdProcesses
  InitPluginsDir
  ClearErrors
  File /oname=$PLUGINSDIR\focusd-control.exe "${BINARY_DIR}\focusd.exe"
  IfErrors shutdown_failed
  DetailPrint "Waiting for Focusd terminals and daemon to exit gracefully..."
  ClearErrors
  ExecWait '"$PLUGINSDIR\focusd-control.exe" shutdown-clients --install-dir "$INSTDIR"' $0
  IfErrors shutdown_failed
  StrCmp $0 "0" shutdown_ready
  shutdown_failed:
    DetailPrint "Focusd could not shut down safely. Close all Focusd terminals and stop the daemon manually, then retry. Older versions must be stopped manually."
    IfSilent +2
      MessageBox MB_OK|MB_ICONEXCLAMATION "Focusd could not shut down safely. Close all Focusd terminals and stop the daemon manually, then retry. Older versions must be stopped manually."
    SetErrorLevel 2
    Abort
  shutdown_ready:
!macroend

Section "Install"
  !insertmacro PrepareFocusdProcesses
  SetOutPath "$INSTDIR"

  File "${BINARY_DIR}\focusd.exe"
  File "${BINARY_DIR}\focusd_daemon.exe"

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
  !insertmacro PrepareFocusdProcesses

  ; Remove from user PATH while the exe still exists. focusd.exe owns the
  ; logic; see the install section.
  IfFileExists "$INSTDIR\focusd.exe" 0 skip_path_disable
    ExecWait '"$INSTDIR\focusd.exe" path disable' $0
    StrCmp $0 "0" path_disable_succeeded
    DetailPrint "WARNING: could not remove focusd from PATH (exit code $0). The PATH backup is retained for recovery."
    Goto skip_path_disable
  path_disable_succeeded:
    DeleteRegValue HKCU "Environment" "Path.focusd-backup"
  skip_path_disable:

  Delete "$INSTDIR\focusd.exe"
  Delete "$INSTDIR\focusd_daemon.exe"
  Delete "$INSTDIR\focusd_start.vbs"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "focusd"
  Delete "$SMSTARTUP\Focus Daemon.lnk"
  Delete "$INSTDIR\uninstall.exe"

  ; Remove registry uninstall keys
  DeleteRegKey HKCU "${UNINST_KEY}"
  DeleteRegKey HKCU "Software\Classes\AppUserModelId\Focusd.Desktop"

  ; $INSTDIR also holds focusd.db and config.json, which is the user's tracking
  ; history. It is deliberately not removed, so uninstall and reinstall keep the
  ; data. The directory is only removable once it is empty anyway.
  RMDir "$INSTDIR"
SectionEnd

