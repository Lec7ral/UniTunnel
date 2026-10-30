# UniTunnel

Cliente de túnel para redes universitarias con proxy Squid autenticado (NTLM/Basic).
Escritorio Windows, interfaz React y backend Go sobre **Wails v3**.

El problema que resuelve: en una red con proxy restrictivo (`10.0.0.1:8080`) solo el
navegador funciona. UniTunnel levanta un túnel `HTTP CONNECT` autenticado contra el
proxy y lo expone a todo el sistema, ya sea por proxy de sistema o mediante un
adaptador virtual Wintun + `tun2socks`.

---

## Estructura del repositorio

| Ruta | Contenido |
|---|---|
| `src/` | Frontend React + TypeScript (Vite). |
| `unitunnel-go/` | Backend Go: servicio Wails, túnel, proxy local, integración con Windows. |
| `unitunnel-go/dist/` | Salida del build del frontend, incrustada en el `.exe` con `//go:embed all:dist`. |
| `dist-release/` | Paquete final (`unitunnel.exe`, `wintun.dll`, `tun2socks.exe`, `config.json`). |
| `.github/workflows/build-windows.yml` | Build reproducible en GitHub Actions. |
| `PLAN.md` | Auditoría y plan de resolución (documento de trabajo). |

---

## Requisitos

- **Windows 10/11 x64**.
- **Go 1.22.4+** (el mínimo que exige `wails/v3 v3.0.0-alpha.8`; la versión exacta está en `unitunnel-go/go.mod`).
- **MinGW-w64 (GCC)** en el `PATH`: Wails v3 necesita cgo en Windows.
- **Node.js 20+** para el frontend.
- Permisos de **Administrador** para crear el adaptador Wintun y modificar rutas/métricas.

---

## Compilar

### Automático (recomendado)

Sube el repositorio a GitHub y deja que corra el workflow
**“Build Windows Application (UniTunnel Wails v3)”**. El artefacto se llama
`unitunnel-windows-wails3-x64` y ya incluye `wintun.dll` y `tun2socks.exe`.

### Local

```cmd
npm ci
npm run build          :: escribe en unitunnel-go\dist
cd unitunnel-go
go mod download
go vet ./...
go build -ldflags="-H windowsgui -s -w" -o ..\dist-release\unitunnel.exe .
```

O simplemente ejecuta `unitunnel-go\build_windows.bat`, que hace todo lo anterior
(incluido descargar `wintun.dll` y `tun2socks.exe`) y deja el paquete en `dist-release\`.

> `npm run build` **debe** ejecutarse antes de `go build`. El binario incrusta
> `unitunnel-go/dist` con `//go:embed all:dist`; si ese directorio solo tiene el
> `.gitkeep`, el `.exe` arranca con una ventana vacía.

---

## Ejecutar

1. Copia `unitunnel.exe`, `wintun.dll` y `tun2socks.exe` a la misma carpeta.
2. Clic derecho → **Ejecutar como administrador**.
3. Configura servidor proxy, cuenta y modo de captura en el panel de ajustes.
4. Pulsa **Conectar**.

La configuración vive en `config.json`, junto al `.exe`. Hay una plantilla en
`unitunnel-go/config.example.json`.

---

## Documentación

- Detalles de funcionamiento, modos de red, seguridad y resolución de problemas:
  [`unitunnel-go/README.md`](unitunnel-go/README.md).
- Estado de la auditoría y trabajo pendiente: [`PLAN.md`](PLAN.md).
