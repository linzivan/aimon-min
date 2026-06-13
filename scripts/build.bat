@echo off
REM ========================================
REM  AI Monitor Build Script
REM ========================================

setlocal enabledelayedexpansion

echo ========================================
echo  Building AI Monitor v1.0.0
echo ========================================

set GO=C:\Program Files\Go\bin\go.exe
set MINGW=C:\msys64\mingw64\bin

set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=1
set CC=%MINGW%\x86_64-w64-mingw32-gcc.exe
set CXX=%MINGW%\x86_64-w64-mingw32-g++.exe
set PATH=%MINGW%;%PATH%

echo.
echo [1/2] Building DEBUG version (with console)...
"%GO%" build ^
    -ldflags="-s -w" ^
    -trimpath ^
    -o "..\AI-Monitor-DEBUG.exe" ^
    .

echo.
echo [2/2] Building RELEASE version (no console)...
"%GO%" build ^
    -ldflags="-H windowsgui -s -w" ^
    -trimpath ^
    -o "..\AI-Monitor.exe" ^
    .

if %ERRORLEVEL% neq 0 (
    echo.
    echo BUILD FAILED! Error code: %ERRORLEVEL%
    exit /b %ERRORLEVEL%
)

echo.
echo Build successful!
dir "..\AI-Monitor*.exe"

echo.
echo Done.
