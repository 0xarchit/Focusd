!ifndef VERSION
  !define VERSION "0.0.0"
!endif

!include "MUI2.nsh"
!include "WinMessages.nsh"


Name "Focusd"
OutFile "focusd_setup.exe"
InstallDir "$APPDATA\focusd"

; Registry key for uninstall info
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Focusd"

; Interface Settings
!define MUI_ABORTWARNING

; Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_WELCOME
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_UNPAGE_FINISH

; Languages
!insertmacro MUI_LANGUAGE "English"

Section "Install"
  SetOutPath "$INSTDIR"
  
  ; Stop existing daemon if running
  DetailPrint "Stopping existing daemon if active..."
  IfFileExists "$INSTDIR\focusd.exe" 0 +3
    ExecWait '"$INSTDIR\focusd.exe" stop'
    Sleep 1000
  ExecWait 'taskkill.exe /F /IM focusd_daemon.exe'
  Sleep 500

  File "focusd.exe"
  File "focusd_daemon.exe"

  ; Delete legacy Startup shortcut if it exists
  Delete "$SMSTARTUP\Focus Daemon.lnk"

  ; Add to user Autostart registry run key
  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "focusd" '"$INSTDIR\focusd_daemon.exe"'

  ; Add to user PATH environment variable via registry
  ReadRegStr $0 HKCU "Environment" "Path"
  Push $0
  Push "$INSTDIR"
  Call ContainsPath
  Pop $1
  IntCmp $1 1 path_done
    StrCmp $0 "" 0 +3
      StrCpy $2 "$INSTDIR"
      Goto write_path
    StrCpy $2 "$0;$INSTDIR"
    write_path:
      WriteRegExpandStr HKCU "Environment" "Path" "$2"
      ; Broadcast environment change to windows shell
      SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000
  path_done:

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
  Sleep 500

  Delete "$INSTDIR\focusd.exe"
  Delete "$INSTDIR\focusd_daemon.exe"
  Delete "$INSTDIR\focusd_start.vbs"
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "focusd"
  Delete "$SMSTARTUP\Focus Daemon.lnk"
  Delete "$INSTDIR\uninstall.exe"

  ; Remove from user PATH
  ReadRegStr $0 HKCU "Environment" "Path"
  Push $0
  Push "$INSTDIR"
  Call un.RemovePath
  Pop $1
  WriteRegExpandStr HKCU "Environment" "Path" "$1"
  SendMessage ${HWND_BROADCAST} ${WM_SETTINGCHANGE} 0 "STR:Environment" /TIMEOUT=5000

  ; Remove registry uninstall keys
  DeleteRegKey HKCU "${UNINST_KEY}"

  RMDir "$INSTDIR"
SectionEnd

; Helper functions for PATH manipulation

Function ContainsPath
  Exch $R0 ; path to search for
  Exch
  Exch $R1 ; full path env var
  Push $R2
  Push $R3
  Push $R4
  Push $R5

  StrLen $R2 $R0
  StrLen $R3 $R1
  StrCpy $R4 0

  loop:
    StrCpy $R5 $R1 $R2 $R4
    StrCmp $R5 $R0 found
    IntOp $R4 $R4 + 1
    IntCmp $R4 $R3 loop loop
    StrCpy $R0 0
    Goto done

  found:
    StrCpy $R0 1

  done:
    Pop $R5
    Pop $R4
    Pop $R3
    Pop $R2
    Pop $R1
    Exch $R0
FunctionEnd

Function un.RemovePath
  Exch $R0 ; path to remove
  Exch
  Exch $R1 ; full path env var
  Push $R2
  Push $R3
  Push $R4
  Push $R5

  StrLen $R2 $R0
  StrCpy $R3 ""
  
  loop:
    StrCpy $R4 0
  find_semi:
    StrCpy $R5 $R1 1 $R4
    StrCmp $R5 "" end_split
    StrCmp $R5 ";" end_split
    IntOp $R4 $R4 + 1
    Goto find_semi

  end_split:
    StrCpy $R5 $R1 $R4
    StrCmp $R5 $R0 skip
    StrCmp $R5 "" skip
    StrCmp $R3 "" first
    StrCpy $R3 "$R3;$R5"
    Goto skip
  first:
    StrCpy $R3 "$R5"
  skip:
    IntOp $R4 $R4 + 1
    StrCpy $R1 $R1 "" $R4
    StrCmp $R1 "" done
    Goto loop

  done:
    StrCpy $R0 $R3

  Pop $R5
  Pop $R4
  Pop $R3
  Pop $R2
  Pop $R1
  Exch $R0
FunctionEnd
