@echo off
setlocal
set "PROJECT_DIR=%~dp0"

where go >nul 2>nul || (echo [ERROR] Go is not installed or not in PATH.& exit /b 1)
where node >nul 2>nul || (echo [ERROR] Node.js is not installed or not in PATH.& exit /b 1)
where npm >nul 2>nul || (echo [ERROR] npm is not installed or not in PATH.& exit /b 1)

if not exist "%PROJECT_DIR%config.toml" (
  copy "%PROJECT_DIR%config.toml.example" "%PROJECT_DIR%config.toml" >nul
  echo Created config.toml from the development template.
)

if "%DOMAINSPRITE_MASTER_KEY%"=="" (
  if exist "%PROJECT_DIR%config.toml.master-key" (
    echo Database master key will be loaded from config.toml.master-key.
  ) else (
    echo Database master key will be generated at config.toml.master-key.
  )
)

if not exist "%PROJECT_DIR%webui\node_modules" (
  echo Installing frontend dependencies...
  call npm ci --prefix "%PROJECT_DIR%webui" || exit /b 1
)

echo Backend:  http://127.0.0.1:2485
echo Frontend: http://127.0.0.1:5173
echo Press Ctrl+C to stop. After a service exits, press R to restart both.
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%PROJECT_DIR%start.ps1"
exit /b %ERRORLEVEL%
