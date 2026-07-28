@echo off
REM Deprecated wrapper. Prefer passing flags to sub2api.exe directly.
REM Example:
REM   sub2api.exe -deploy-mode=diy -auto-setup -admin-email=admin@example.com -admin-password=YourPass -port=8080
REM   sub2api.exe -config=config.yaml
REM   sub2api.exe -h
cd /d "%~dp0"
if not exist sub2api.exe (
  echo ERROR: sub2api.exe not found
  exit /b 1
)
sub2api.exe %*
if errorlevel 1 (
  echo.
  echo Run: sub2api.exe -h
)
