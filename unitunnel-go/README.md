# UniTunnel - Gestor de Túnel para Proxy Universitario (Windows y Android)

UniTunnel es una solución nativa escrita en **Go** utilizando la librería gráfica **Fyne v2**, el adaptador virtual de red **Wintun** y un cliente de túnel `HTTP CONNECT` con soporte completo para autenticación corporativa **NTLM (v1/v2)** y **Basic**.

Está diseñado para resolver el problema clásico de las redes universitarias con proxies restringidos (`10.0.0.1:8080`), donde las aplicaciones de escritorio (Git, Discord, Steam, terminal, juegos) quedan bloqueadas mientras que el navegador sí funciona.

---

## 🚀 Compilación Automática con GitHub Actions (Recomendado)

No necesitas instalar Go ni compiladores C en tu máquina si usas el workflow de GitHub Actions incluido en este repositorio:

1. Sube este repositorio a tu cuenta de GitHub (`git push origin main`).
2. Ve a la pestaña **Actions** en tu repositorio de GitHub.
3. El workflow **"Build Windows Application (UniTunnel)"** se ejecutará automáticamente.
4. Al terminar (aprox. 2-3 minutos), haz clic en la ejecución y descarga el artefacto **`unitunnel-windows-x64.zip`**.
5. Descomprime el `.zip`. Contiene:
   - `unitunnel.exe` (ejecutable nativo)
   - `wintun.dll` (driver oficial de WireGuard/Wintun)
   - `config.json` (perfil con 10.0.0.1:8080)
   - `LEEME.txt`

---

## 🛠️ Compilación Manual en Windows (Local)

### Requisitos:
1. **Go 1.22+** instalado ([golang.org](https://go.dev/dl/)).
2. **MinGW-w64 (GCC)** agregado al `PATH` de Windows (requerido por Fyne para compilar OpenGL/Win32).
   - Puedes instalarlo rápidamente con WinGet: `winget install -e --id MSYS2.MSYS2` o descargando [w64devkit](https://github.com/skeeto/w64devkit/releases).
3. **`wintun.dll`** (versión amd64 de [wintun.net](https://www.wintun.net/)).

### Pasos:
```cmd
cd unitunnel-go
go mod tidy
go build -ldflags="-H windowsgui -s -w" -o unitunnel.exe ./cmd/unitunnel
```

Coloca `wintun.dll` en la misma carpeta que `unitunnel.exe`.

---

## ⚡ Ejecución en Windows

> [!IMPORTANT]
> **SIEMPRE haz clic derecho en `unitunnel.exe` y selecciona "Ejecutar como administrador".**
> Los adaptadores virtuales de red y la modificación de rutas IP de Windows requieren privilegios elevados.

### Pasos de Uso:
1. Abre **UniTunnel**.
2. Verifica la IP del proxy (`10.0.0.1`) y el puerto (`8080`).
3. Ingresa tu usuario y contraseña institucional.
4. Pulsa el botón **"Diagnosticar Proxy (10.0.0.1)"**: UniTunnel enviará una sonda y detectará automáticamente si requiere NTLM o Basic.
5. Selecciona el modo:
   - **Modo TUN Virtual (Wintun)**: Habilita el adaptador virtual. Todo el tráfico de la PC (Git, juegos, terminal, navegadores) pasará por el túnel.
   - **Modo Proxy de Sistema**: Solo cambia la configuración de red de Windows (ideal si no tienes permisos de administrador).
6. Pulsa **"CONECTAR TÚNEL"**.

---

## 📱 Compilación para Android

Para generar el `.apk` para Android:
1. Instala `fyne-cross`:
   ```bash
   go install fyne.io/fyne/v2/cmd/fyne-cross@latest
   ```
2. Requiere Docker para compilar el toolchain de Android NDK:
   ```bash
   fyne-cross android -app-id com.uninet.unitunnel -icon assets/icon.png ./cmd/unitunnel
   ```
   El archivo `.apk` resultante se guardará en `fyne-cross/dist/android/`.
