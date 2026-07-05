@echo off
setlocal

go build -ldflags="-s -w -X 'focusd/system.Version=dev-build'" -trimpath -o focusd.exe ./cmd/focusd
if errorlevel 1 exit /b %errorlevel%

go build -ldflags="-s -w -H=windowsgui -X 'focusd/system.Version=dev-build'" -trimpath -o focusd_daemon.exe ./cmd/focusd_daemon
if errorlevel 1 exit /b %errorlevel%

makensis /DVERSION=dev-build installer.nsi
if errorlevel 1 exit /b %errorlevel%

echo Build complete.
exit /b 0
