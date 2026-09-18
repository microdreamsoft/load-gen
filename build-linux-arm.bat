@echo off
setlocal

set "ROOT_DIR=%~dp0"
if defined GOARCH (
    set "ARCH=%GOARCH%"
) else (
    set "ARCH=arm64"
)

if /i "%ARCH%"=="arm" goto valid_arch
if /i "%ARCH%"=="arm64" goto valid_arch
echo unsupported GOARCH: %ARCH% (expected arm or arm64)>&2
exit /b 1

:valid_arch
if not defined OUTPUT set "OUTPUT=%ROOT_DIR%load-gen-linux-%ARCH%"

cd /d "%ROOT_DIR%"
set "GOOS=linux"
set "GOARCH=%ARCH%"
set "CGO_ENABLED=0"
go build -trimpath -ldflags "-s -w" -o "%OUTPUT%" .
if errorlevel 1 exit /b %errorlevel%

echo built linux/%ARCH%: %OUTPUT%