@echo off
setlocal enabledelayedexpansion

echo ========================================================
echo   Compilando UniTunnel para Windows (x64) con Icono
echo ========================================================

cd /d "%~dp0"

:: 1. Verificar si windres está disponible para compilar recursos
where windres >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    echo [*] Compilando recursos PE (icono + manifiesto de Administrador)...
    if exist "cmd\unitunnel\unitunnel.rc" (
        cd cmd\unitunnel
        windres -i unitunnel.rc -O coff -o rsrc_windows_amd64.syso
        cd ..\..
    ) else if exist "unitunnel.rc" (
        windres -i unitunnel.rc -O coff -o rsrc_windows_amd64.syso
    )
) else (
    echo [i] windres no encontrado, utilizando rsrc_windows_amd64.syso precompilado.
)

:: 2. Crear directorio dist si no existe
if not exist "..\dist" mkdir "..\dist"

:: 3. Descargar wintun.dll si no existe en dist
if not exist "..\dist\wintun.dll" (
    echo [*] Descargando wintun.dll oficial para 64 bits...
    powershell -Command "Invoke-WebRequest -Uri 'https://www.wintun.net/builds/wintun-0.14.1.zip' -OutFile '..\dist\wintun.zip'; Expand-Archive -Path '..\dist\wintun.zip' -DestinationPath '..\dist\temp_wintun' -Force; Copy-Item '..\dist\temp_wintun\wintun\bin\amd64\wintun.dll' '..\dist\wintun.dll' -Force; Remove-Item -Recurse -Force '..\dist\temp_wintun', '..\dist\wintun.zip'"
)

:: 4. Habilitar CGO para OpenGL y Win32
set CGO_ENABLED=1

:: 5. Compilar ejecutable sin consola de fondo (-H windowsgui)
echo [*] Compilando binario unitunnel.exe con icono incrustado...
if exist "cmd\unitunnel\main.go" (
    go build -ldflags="-H windowsgui -s -w" -o ..\dist\unitunnel.exe .\cmd\unitunnel
) else (
    go build -ldflags="-H windowsgui -s -w" -o ..\dist\unitunnel.exe .
)

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ========================================================
    echo   Compilacion exitosa: dist\unitunnel.exe
    echo   - Icono incrustado en el ejecutable .exe
    echo   - Manifiesto UAC RequireAdministrator activo
    echo   - Menu de bandeja del sistema dinamico (Conectar/Desconectar)
    echo ========================================================
    if not exist "..\dist\config.json" copy "config.example.json" "..\dist\config.json" >nul
) else (
    echo.
    echo [!] Error durante la compilacion. Verifica tener Go 1.22+ y MinGW GCC instalados.
)

pause
