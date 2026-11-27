@echo off
setlocal
cd /d "%~dp0"
if not exist "Port8080.exe" (
  echo Port8080.exe is missing. Re-download or re-extract the package.
  pause
  exit /b 1
)
Port8080.exe
if errorlevel 1 (
  echo.
  echo Port8080 stopped with an error. Check the messages above.
  pause
)
