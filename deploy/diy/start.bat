@echo off
cd /d "%~dp0"

REM Do NOT copy config.example.yaml -> config.yaml here.
REM A pre-copied config skips AutoSetup and leaves users table empty.

if not exist .env if exist .env.example copy /Y .env.example .env >nul

set DEPLOY_MODE=diy
set DATABASE_DRIVER=sqlite
set DATABASE_PATH=./data/sub2api.db
set REDIS_EMBEDDED=true
set AUTO_SETUP=true
set TZ=UTC

if not exist data mkdir data

echo.
echo === Sub2API DIY ===
echo SQLite + embedded Redis  -  http://127.0.0.1:8080
echo Working dir: %CD%
echo.
echo First run creates admin from .env ADMIN_EMAIL / ADMIN_PASSWORD
echo (or prints a one-time password if ADMIN_PASSWORD is empty).
echo.

if not exist sub2api.exe (
  echo ERROR: sub2api.exe not found in this folder.
  pause
  exit /b 1
)

sub2api.exe
echo.
echo Process exited with code %ERRORLEVEL%
pause
