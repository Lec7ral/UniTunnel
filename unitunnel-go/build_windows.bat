@echo off
setlocal enabledelayedexpansion

echo ========================================================
echo   Compilando UniTunnel para Windows (x64)
echo ========================================================

cd /d "%~dp0"
for %%I in ("%~dp0..") do set "ROOT=%%~fI"
set "RELEASE=%ROOT%\dist-release"

:: Wails v3 needs cgo on Windows (WebView2/Win32 bindings).
set CGO_ENABLED=1

:: ---------------------------------------------------------------------------
:: 1. Frontend React (Vite) -> unitunnel-go\dist
::    Vite escribe directamente en unitunnel-go\dist, que es lo que incrusta
::    `//go:embed all:dist` (unitunnel-go\main.go). No hay copia intermedia.
:: ---------------------------------------------------------------------------
echo.
echo [1/6] Compilando el frontend React (Vite)...
if not exist "%ROOT%\node_modules" (
    echo [ERROR] Falta node_modules. Ejecuta primero:  npm ci
    pause
    exit /b 1
)
pushd "%ROOT%"
call npm run build
if errorlevel 1 (
    popd
    echo [ERROR] Error compilando el frontend.
    pause
    exit /b 1
)
popd

if not exist "dist\index.html" (
    echo [ERROR] No se genero unitunnel-go\dist\index.html; el .exe quedaria sin interfaz.
    pause
    exit /b 1
)

:: ---------------------------------------------------------------------------
:: 2. Recursos PE (icono + manifiesto UAC). El .syso esta versionado, asi que
::    windres es solo un refresco opcional.
:: ---------------------------------------------------------------------------
echo.
echo [2/6] Recursos PE...
where windres >nul 2>nul
if errorlevel 1 (
    echo [i] windres no encontrado; se usa rsrc_windows_amd64.syso precompilado.
) else (
    if exist "unitunnel.rc" (
        echo [*] Compilando recursos con windres...
        windres -i unitunnel.rc -O coff -o rsrc_windows_amd64.syso
    )
)

:: ---------------------------------------------------------------------------
:: 3. Carpeta de distribucion
:: ---------------------------------------------------------------------------
if not exist "%RELEASE%" mkdir "%RELEASE%"

:: ---------------------------------------------------------------------------
:: 4. Dependencias externas del modo tun (wintun.dll + tun2socks.exe)
:: ---------------------------------------------------------------------------
echo.
echo [3/6] Dependencias externas...
if not exist "%RELEASE%\wintun.dll" (
    echo [*] Descargando wintun.dll oficial para 64 bits...
    pushd "%RELEASE%"
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Invoke-WebRequest -Uri 'https://www.wintun.net/builds/wintun-0.14.1.zip' -OutFile 'wintun.zip'; Expand-Archive -Path 'wintun.zip' -DestinationPath 'temp_wintun' -Force; Copy-Item 'temp_wintun\wintun\bin\amd64\wintun.dll' 'wintun.dll' -Force; Remove-Item -Recurse -Force 'temp_wintun', 'wintun.zip'" || echo [ERROR] No se pudo descargar wintun.dll; copialo a mano a dist-release.
    popd
)

if not exist "%RELEASE%\tun2socks.exe" (
    echo [*] Descargando tun2socks (motor L3)...
    pushd "%RELEASE%"
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Invoke-WebRequest -Uri 'https://github.com/xjasonlyu/tun2socks/releases/download/v2.5.2/tun2socks-windows-amd64.zip' -OutFile 'tun2socks.zip'; Expand-Archive -Path 'tun2socks.zip' -DestinationPath 'temp_tun2socks' -Force; Copy-Item 'temp_tun2socks\tun2socks-windows-amd64.exe' 'tun2socks.exe' -Force; Remove-Item -Recurse -Force 'temp_tun2socks', 'tun2socks.zip'" || echo [ERROR] No se pudo descargar tun2socks.exe; copialo a mano a dist-release.
    popd
)

:: ---------------------------------------------------------------------------
:: 5. Verificacion y compilacion del binario
:: ---------------------------------------------------------------------------
echo.
echo [4/6] Verificando dependencias Go (go mod download)...
if not exist "go.sum" (
    echo [WARN] No existe go.sum. Se intentara generar con go mod download.
    echo        Si falla, ejecuta una vez con red:  go mod tidy   y commitea go.sum.
)
go mod download
if errorlevel 1 (
    echo [ERROR] Fallo go mod download. Si el error es "missing go.sum entry",
    echo     ejecuta una vez con red:  go mod tidy   y commitea go.sum.
    pause
    exit /b 1
)

echo.
echo [5/6] go vet ./...
go vet ./...
if errorlevel 1 (
    echo [ERROR] go vet encontro problemas.
    pause
    exit /b 1
)

echo.
echo [6/6] Compilando unitunnel.exe (-H windowsgui)...
go build -ldflags="-H windowsgui -s -w" -o "%RELEASE%\unitunnel.exe" .
if errorlevel 1 (
    echo.
    echo [ERROR] Error durante la compilacion. Verifica tener Go 1.22+ y MinGW GCC instalados.
    pause
    exit /b 1
)

:: ---------------------------------------------------------------------------
:: 6. Paquete final
:: ---------------------------------------------------------------------------
if not exist "%RELEASE%\config.json" copy "config.example.json" "%RELEASE%\config.json" >nul
if exist "icon.ico" copy "icon.ico" "%RELEASE%\icon.ico" >nul

echo.
echo ========================================================
echo   Compilacion exitosa: dist-release\unitunnel.exe
echo   - Icono incrustado en el ejecutable .exe
echo   - Manifiesto UAC RequireAdministrator activo
echo   - wintun.dll y tun2socks.exe copiados a dist-release
echo ========================================================
pause
