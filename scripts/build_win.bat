@echo off
setlocal
set CGO_ENABLED=1
set CC=C:\msys64\mingw64\bin\x86_64-w64-mingw32-gcc.exe
set GOOS=windows
set GOARCH=amd64
set PATH=C:\msys64\mingw64\bin;%PATH%
cd /d D:\ai_code\ai_monitor

echo [1/2] go mod tidy...
call "C:\Program Files\Go\bin\go.exe" mod tidy

echo.
echo [2/2] Building DEBUG + RELEASE...
call "C:\Program Files\Go\bin\go.exe" build -ldflags="-s -w" -trimpath -o AI-Monitor-DEBUG.exe .
if %ERRORLEVEL% neq 0 (
    echo DEBUG BUILD FAILED: %ERRORLEVEL%
    exit /b %ERRORLEVEL%
)

call "C:\Program Files\Go\bin\go.exe" build -ldflags="-H windowsgui -s -w" -trimpath -o AI-Monitor.exe .
if %ERRORLEVEL% neq 0 (
    echo RELEASE BUILD FAILED: %ERRORLEVEL%
    exit /b %ERRORLEVEL%
)

echo.
echo BUILD SUCCESSFUL!
dir D:\ai_code\ai_monitor\AI-Monitor*.exe
