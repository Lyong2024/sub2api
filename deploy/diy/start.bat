@echo off
cd /d "%~dp0"
if not exist .env if exist .env.example copy /Y .env.example .env >nul
if not exist config.yaml if exist config.example.yaml copy /Y config.example.yaml config.yaml >nul
set DEPLOY_MODE=diy
set DATABASE_DRIVER=sqlite
set REDIS_EMBEDDED=true
set AUTO_SETUP=true
echo Starting Sub2API DIY (SQLite + embedded Redis) on http://127.0.0.1:8080 ...
if exist sub2api.exe (
  sub2api.exe
) else (
  echo sub2api.exe not found in this folder.
  pause
  exit /b 1
)
pause
