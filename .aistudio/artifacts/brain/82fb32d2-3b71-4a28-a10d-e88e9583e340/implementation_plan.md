# UniTunnel: Solución Nativa en Go + Fyne con GitHub Actions para Windows

Una solución completa y autocontenida en el repositorio para superar las restricciones del proxy universitario (`10.0.0.1:8080`), lista para ser compilada automáticamente en la nube mediante **GitHub Actions** para Windows (generando `unitunnel.exe` con `wintun.dll`), junto a la suite interactiva de configuración y diagnóstico.

> [!IMPORTANT]
> **Integración Total en el Repositorio para GitHub Actions**:
> Todo el código fuente de Go (`main.go`, paquetes de red, adaptadores Wintun, motor tun2socks, Fyne GUI), archivos de configuración y el workflow de CI/CD de GitHub (`.github/workflows/build-windows.yml`) estarán creados directamente dentro del repositorio. 
> Cuando subas el código a tu repositorio de GitHub, la acción de compilación se ejecutará de forma automática en máquinas virtuales de Windows (`windows-latest`), configurará MinGW/GCC (Cgo), descargará la DLL oficial de Wintun, compilará el ejecutable nativo `.exe` y generará un artefacto `.zip` listo para descargar y ejecutar en tu PC.

---

### Decisiones de Arquitectura Confirmadas

- **Entorno de Compilación**: Pipeline automatizado con **GitHub Actions** (`.github/workflows/build-windows.yml`) que instala Go 1.22+, MinGW-w64 (necesario para Cgo en Fyne), compila sin ventana de consola (`-ldflags="-H windowsgui -s -w"`), empaqueta `wintun.dll` y sube el `.zip` como Artifact de GitHub.
- **Captura Dual de Tráfico**:
  1. *Modo TUN Virtual (Wintun)*: Enruta el 100% del tráfico del sistema (Git, juegos, Discord, terminal, navegadores) a través de un adaptador de red virtual de alto rendimiento.
  2. *Modo Proxy de Sistema (WinINet)*: Modifica la configuración de proxy de Windows a `127.0.0.1:1080` para uso sin privilegios de administrador.
- **Mecanismo de Salida Egress**:
  - *Modo Local Directo*: Mediante peticiones `HTTP CONNECT` autenticadas contra `10.0.0.1:8080` (sin servidores externos).
  - *Modo VPS*: Mediante encapsulación en WebSocket seguro (`wss://`) o SSH a través del proxy hacia un VPS propio para obtener UDP completo y DNS privado.
- **Detección Automática de Autenticación**:
  - Sonda inicial que inspecciona las cabeceras `Proxy-Authenticate` y resuelve automáticamente **NTLM (v1/v2)**, **Basic** o **Digest**.

---

## 1. Estructura de Archivos en el Repositorio

El repositorio contendrá tanto el código Go listo para GitHub Actions como la suite web interactiva:

```
├── .github/
│   └── workflows/
│       └── build-windows.yml      # CI/CD: Compila unitunnel.exe en GitHub Actions
├── unitunnel-go/                  # Proyecto nativo en Go + Fyne
│   ├── cmd/
│   │   └── unitunnel/
│   │       └── main.go            # Entry point y arranque de Fyne v2
│   ├── pkg/
│   │   ├── auth/
│   │   │   ├── detector.go        # Detección automática de NTLM/Basic
│   │   │   └── ntlm.go            # Handshake NTLM Type-1/2/3
│   │   ├── tunnel/
│   │   │   ├── connect.go         # Cliente HTTP CONNECT bidireccional
│   │   │   ├── socks5.go          # Servidor SOCKS5 local en 127.0.0.1:1080
│   │   │   └── vps.go             # Túnel WebSocket/SSH para VPS
│   │   ├── tun/
│   │   │   ├── wintun_windows.go  # Carga de Wintun.dll y tablas de ruta
│   │   │   └── tun2socks.go       # Stack gVisor netstack (IP -> TCP)
│   │   ├── sysproxy/
│   │   │   └── proxy_windows.go   # Control de proxy WinINet en Windows
│   │   └── config/
│   │       └── config.go          # Lectura y guardado de perfil (config.json)
│   ├── ui/
│   │   ├── gui.go                 # Interfaz visual Fyne (botones, estados, switch)
│   │   └── theme.go               # Estilos visuales de la aplicación
│   ├── go.mod                     # Dependencias Go (Fyne, netstack, etc.)
│   └── config.example.json        # Configuración inicial con 10.0.0.1:8080
├── src/                           # Suite web interactiva (visión general, generador y simulador)
│   ├── App.tsx
│   └── components/
└── metadata.json
```

---

## 2. Configuración del Flujo en GitHub Actions

El archivo `.github/workflows/build-windows.yml` ejecutará los siguientes pasos:
1. `runs-on: windows-latest`
2. Checkout del repositorio.
3. Instalación de **Go 1.22+**.
4. Verificación del compilador C (GCC en MinGW para habilitar `CGO_ENABLED=1`, requisito indispensable de Fyne).
5. Descarga automática de la librería oficial de WireGuard `wintun.dll` (versión x64).
6. Ejecución de `go build -ldflags="-H windowsgui -s -w" -o dist/unitunnel.exe ./cmd/unitunnel`.
7. Empaquetado de `dist/` en `unitunnel-windows-x64.zip` conteniendo:
   - `unitunnel.exe`
   - `wintun.dll`
   - `config.json`
   - `INSTRUCCIONES.txt`
8. `actions/upload-artifact@v4`: Publica el `.zip` en la pestaña de Actions para descarga inmediata con un clic.

---

## 3. Arquitectura del Código Go

```
┌────────────────────────────────────────────────────────────────────────┐
│                        Windows Applications                            │
│  [Browser]   [Git / CLI]   [Games / Discord]   [System Services]       │
└────────────┬──────────────┬───────────────────┬────────────────────────┘
             │              │                   │
             ▼              ▼                   ▼
┌────────────────────────────────────────────────────────────────────────┐
│   Captura: Wintun Network Adapter (Layer 3 IP Packets)                 │
└────────────────────────────────────┬───────────────────────────────────┘
                                     │ Raw IP Packets
                                     ▼
┌────────────────────────────────────────────────────────────────────────┐
│   Motor Go: tun2socks con gVisor Netstack                              │
│   Convierte paquetes IP en flujos TCP/UDP y resuelve DNS con Fake-IP   │
└────────────────────────────────────┬───────────────────────────────────┘
                                     │
                                     ▼
┌────────────────────────────────────────────────────────────────────────┐
│   Handshake y Autenticador NTLM / Basic                                │
│   Inspecciona cabeceras del proxy 10.0.0.1:8080 y autentica            │
└────────────────────────────────────┬───────────────────────────────────┘
                                     │ HTTP CONNECT
                                     ▼
┌────────────────────────────────────────────────────────────────────────┐
│   Proxy Universitario (10.0.0.1:8080)                                  │
│   -> 200 Connection Established                                        │
└───────────────────┬────────────────────────────────┬───────────────────┘
                    │                                │
                    ▼                                ▼
       [Destinos Directos 443]             [VPS Túnel Exit-Node]
```

---

## 4. Plan de Implementación y Verificación

1. **Creación del Proyecto Go Completo**:
   - Crear `unitunnel-go/go.mod` con las librerías necesarias (`fyne.io/fyne/v2`, `golang.org/x/sys/windows`, etc.).
   - Crear todos los paquetes funcionales (`cmd/unitunnel/main.go`, `pkg/auth`, `pkg/tunnel`, `pkg/tun`, `pkg/sysproxy`, `ui/gui.go`).
2. **Creación del Workflow de GitHub Actions**:
   - Escribir `.github/workflows/build-windows.yml` con el pipeline optimizado para Windows.
3. **Suite Web Interactiva**:
   - Desarrollar la aplicación web en React/Tailwind como panel de control complementario:
     - Explorador interactivo del código Go con descripciones pedagógicas.
     - Generador y personalizador de `config.json` para tu usuario y contraseña.
     - Simulador de diagnóstico del proxy (paso a paso del handshake NTLM).
     - Guía paso a paso para ejecutar el GitHub Action y correr `unitunnel.exe` en Windows como administrador.
4. **Verificación y Compilación**:
   - Validar sintaxis del código Go y compilar la suite web con `compile_applet`.
