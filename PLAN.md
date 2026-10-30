# Plan de resolución — UniTunnel

> Documento de trabajo derivado de la auditoría estática del repositorio (commit `e1caafd`).
> Referencias en formato `ruta:línea`. El avance real está en **Estado de ejecución**.

## Diagnóstico en una línea

La app **no funciona hoy**: el frontend nunca invoca al backend, el toggle de hotspot se autobloquea y el build no es reproducible. Todo lo demás (seguridad de red, concurrencia, settings cosméticos) es deuda que solo se puede atacar cuando esos tres estén cerrados.

**Estrategia:** 6 fases, 6 PRs, ordenadas para que cada una sea verificable con la anterior ya en `main`. Estimación total ≈ **45–60 h** de trabajo efectivo (sin contar validación manual en red UCI real).

---

## Estado de ejecución

Última actualización: **2026-10-29**. Leyenda: ✅ hecho · 🟡 parcial · ⛔ bloqueado por el entorno · ⏳ pendiente.

> **Aviso importante:** en el entorno donde se aplicaron estos cambios **no se pudo compilar ni ejecutar nada** (ver *Bloqueos del entorno*). Todo lo marcado ✅ significa "código corregido y revisado contra el código fuente de Wails v3.0.0-alpha.8", **no** "verificado en ejecución".

### PR-1 `fix/critical`

| ID | Estado | Detalle |
|---|---|---|
| **F0.1** | ⛔ | `go.sum` sigue sin existir: el `.zip` de `wails v3.0.0-alpha.8` no está en la caché local y `proxy.golang.org` responde 403. Hay que ejecutar `go mod tidy` en una máquina con red Go y commitear `go.sum`. |
| **F0.2** | ✅ | `go.mod` pasa a `go 1.22.4` (lo exige el `go.mod` de alpha.8; con `1.22.0` el toolchain rechaza el módulo). CI usa `go-version-file: unitunnel-go/go.mod`. Se descartó la línea `toolchain` a propósito: forzaría descargar un toolchain concreto sin necesidad. |
| **F0.3** | ✅ | Vite escribe en `unitunnel-go/dist` (`build.outDir`), que es exactamente lo que incrusta `//go:embed all:dist`. Se añadió `unitunnel-go/dist/.gitkeep` y un plugin de Vite que lo recrea tras `emptyOutDir`, para que un clone limpio compile y `git status` quede limpio. El `.bat` compila el frontend si falta y deja el release en `dist-release/` (igual que CI). |
| **F0.4** | ✅ | Eliminadas las ramas `cmd\unitunnel` del `.bat` y del `README`; ahora usan `unitunnel.rc` / `rsrc_windows_amd64.syso` de la raíz del módulo. |
| **F0.5** | ✅ | CI: `go vet ./...` + `go build ./...` antes del binario, `npm run lint` antes de `npm run build`, fuera `go mod tidy` y fuera el `Copy-Item ..\dist`. ⚠️ El job seguirá en rojo hasta cerrar F0.1 y regenerar `package-lock.json`. |
| **F1.1** | ✅ | `wailsBridge.ts` reescrito: `import { Call } from '@wailsio/runtime'` y `Call.ByName("unitunnel/pkg/service.AppService.<Método>")`. Verificado en el código de alpha.8: `ByName` existe, `CallOptions.MethodName` se resuelve contra `BoundMethod.String()` = `pkgPath.TypeName.Name`, y `main.go` registra `*service.AppService`. Sin `localStorage` como fuente de verdad; sin fallbacks silenciosos. |
| **F1.2** | ✅ | `ToggleForceOfflineHotspot` ya no retiene `s.mu` mientras loguea (deadlock no reentrante); nuevo helper `setHotspotForced`. |
| **F1.3** | ✅ | Eliminados los "éxitos" fabricados en `saveConfig`, `checkHotspotCapability`, `toggleForceOfflineHotspot` y `stopTunnel`. `applyNetworkMode`/`setAlwaysOnTop` ya no hacen un `SaveConfig` extra. `SettingsDrawer` ahora muestra "No se guardó" cuando el backend rechaza la escritura. |
| **F1.4** | ✅ | `main.go`: `return` temprano si `win == nil`; el crash report imprime `Time:` real (RFC3339) y la ruta del log. |

### PR-2 `fix/network-safety` (adelantado)

| ID | Estado | Detalle |
|---|---|---|
| **F2.1** | ✅ | Nuevo `pkg/sysproxy/snapshot_windows.go`: antes de tocar el registro se guarda `ProxyEnable`, `ProxyServer`, `ProxyOverride`, `AutoConfigURL` y las 6 variables `*_proxy` de `HKCU\Environment` en `%LOCALAPPDATA%\UniTunnel\proxy_state.json` (escritura atómica, `0600`). `EnableSystemProxy` **falla cerrado** si no puede guardar el snapshot. `DisableSystemProxy` restaura valor por valor (borrando los que no existían) en vez de apagar el proxy y destruir un AutoConfigURL corporativo. |
| **F2.2** | ✅ | Nuevo `pkg/network/metrics_state_windows.go`: snapshot por interfaz (`if_index`, métrica, `AutomaticMetric`) antes del primer cambio de modo; `ModeDirect` restaura los valores exactos y, solo si no hay snapshot, cae a `AutomaticMetric Enabled`. `GetInterfaces` ahora rellena `InterfaceIndex`, `Status` real (`ConnectionState`) y detecta correctamente `AutomaticMetric` (`Enabled`/`Disabled`, antes siempre `false`). |
| **F2.3** | ✅ | `pkg/tun/routes_state_windows.go`: solo se borran las rutas que UniTunnel registró (`routes_state.json`, escritura atómica tras cada `route add` aceptado). Se eliminaron los `route delete 10.0.0.0 mask 255.0.0.0` indiscriminados de `CleanStaleRoutes` y `Close`. `InspectNetworkState` ya no marca rutas de túnel por simple substring (siempre encontraba `0.0.0.0`) y detecta el estado huérfano por el fichero de propiedad. |
| **F2.4** | ✅ | `main.go` ejecuta `tun.PerformStartupDiagnosticsAndClean` + `network.RestoreMetricsStateIfAny()` **antes** de aplicar el modo guardado: un cierre abrupto ya no deja proxy/rutas/métricas a medias. |
| **F2.5** | 🟡 | `config.SaveConfig` escribe atómico con `0600`; `LoadConfig` guarda `config.json.corrupt-<ts>` y devuelve error en vez de sustituirlo por defaults en silencio; `quota.go` sin `InsecureSkipVerify`, sin fallback a HTTP con Basic y rechaza credenciales sobre `http://`. ⏳ Pendiente: cifrar las contraseñas en reposo con DPAPI (hoy siguen en claro, solo protegidas por permisos de fichero). |
| **F2.6** | ✅ | `pkg/logger/logger.go` reescrito: stdout va **el último** y con errores ignorados (en `-H windowsgui` no hay handle válido y `io.MultiWriter` abortaba todas las escrituras a fichero), rotación a 5 MB (`.1`), permisos `0600` y `Close()` en `main.go` (`defer`). |

### Adelantos de otras fases

| ID | Estado | Detalle |
|---|---|---|
| **F3.1** | ✅ | Máquina de estados real `idle → connecting → connected → stopping` (`connState`) en `AppService`. Un `StopTunnel` durante el arranque gana sobre el `StartTunnel` que aún está terminando (se revalida el estado antes y después de activar el proxy, y se revierte si hubo stop). |
| **F3.2** | ✅ | `handleConnectToggle` ignora clicks mientras `connecting`; el botón central queda `disabled` en ese estado. |
| **F3.3** | ✅ | `SetNetworkMode`, `StartTunnel` y `StopTunnel` del tray corren en goroutine; el menú ya no se congela. ⚠️ Los callbacks de estado tocan `MenuItem.SetLabel`/`SetChecked` desde esa goroutine: verificar en la prueba manual que Wails v3 los marshalea al hilo de UI (si no, habría que envolverlos con `application.InvokeSync`). |
| **F3.4** | ✅ | `DoHServer` acepta la lista de resolvers (`cfg.TunDNS` → `https://<ip>/dns-query`) en vez de hardcodear Cloudflare/Google. Si el listener UDP 53 no arranca en modo `tun`, `Engine.Start` aborta y cierra SOCKS5/HTTP ya abiertos (en `system_proxy` solo avisa, porque ahí resuelve el proxy). |
| **F3.5** | ✅ | `CountingPipe` y `Pipe` propagan `CloseWrite` al primer EOF y esperan ambas direcciones; solo cierran ambos sockets si el primer resultado es un error real. Una descarga con half-close del cliente ya no se trunca. |
| **F3.6** | ✅ | `CountingPipe` invierte la semántica: `read_mb` = descarga (`remote → local`), `write_mb` = subida. |
| **F3.7** | ✅ | `GetTrafficStats` ya no abre un TCP a Squid en cada tick: un prober en background mide el RTT cada 10 s (`startPingProbe`/`stopPingProbe`). |
| **F3.8** | ✅ | Nuevo `pkg/singleinstance`: mutex con nombre + `FindWindowW`/`SetForegroundWindow`; la segunda instancia enfoca la primera y sale. |
| **F4.1** | ✅ | Nuevo `pkg/autostart`: clave `HKCU\...\Run` con la ruta entrecomillada del `.exe`, sincronizada desde `main.go` y desde `SaveConfig`. |
| **F4.2** | ✅ | Timer real de cuota con `context` cancelable según `quota_check_interval_min` (antes solo había un check único al segundo de conectar). |
| **F4.3** | ✅ | `OpenWintun` usa `cfg.BypassRoutes` (CIDR → destino/máscara) en vez de las tres rutas hardcodeadas; resuelve `ProxyHost` si es un nombre. |
| **F4.4** | ✅ | Eliminado el VPS fantasma: `tunnel/vps.go`, campos `vps_*` de `Config` y de la UI, y menciones en `audit.go`. |
| **F4.5** | ✅ | La UI dice `config.json (junto a unitunnel.exe)` en vez de `%USERPROFILE%\UniTunnel\config.json`. |
| **F4.6** | ✅ | Nuevo `Config.Validate()` (host/puertos, puertos locales distintos, modo de captura, IPv4/máscara, CIDR de bypass, URL de cuota, intervalo y cuentas duplicadas) ejecutado antes de escribir `config.json`. |
| **F4.7** | ✅ | Nuevo `src/services/configDiff.ts` (`reconnectReasons`/`requiresReconnect`/`reconnectWarning`). El drawer muestra un aviso ámbar con los ajustes que solo se aplican al reconectar y, si el túnel está activo, `App.tsx` lanza un toast al guardar. Los campos que se leen una sola vez al construir el túnel (proxy, puertos, cuenta, captura, Wintun, bypass, cuota) quedan cubiertos. |
| **F4.8** | ✅ | Sin `yoanbrp` en `wailsBridge.ts`, `SettingsDrawer.tsx` ni `App.tsx`. |
| **F4.9** | ✅ | Eliminado el modal de diagnóstico NTLM (era inalcanzable y no probaba NTLM: solo llamaba a `IsConnected` + `CheckQuota`), eliminado el estado `interfaces`, velocidad formateada con `toFixed(2)` y banner rojo de `backendError` cuando el backend Go no responde. |
| **F4.10** | ✅ | Nuevo `Dialer.DoHTTP`: el HTTP en claro se reenvía con URI absoluta a Squid (con reintento NTLM Type-3/Basic tras 407) en vez de abrir `CONNECT host:80`, que el ACL deniega. Cuerpo limitado a 4 MB para poder reintentar. |

### Fase 5 adelantada (limpieza, docs, CI, tests)

| ID | Estado | Detalle |
|---|---|---|
| **F5.1** | ✅ | Código muerto eliminado tras verificarlo con búsqueda recursiva en todo el módulo: `pkg/tunnel/engine.go` (un segundo `NewEngine` que nadie llamaba y que se confundía con `tun.NewEngine`), `pkg/tunnel/audit.go` (`RunProxyAudit` inalcanzable), `pkg/tun/prober.go` (`CheckInternetFlow`/`StartPeriodicPing`), `traffic.StartSpeedMonitor` y la variable `currentSpeedMBs`. `pkg/auth/detector.go` se redujo a `pkg/auth/types.go` (solo `AuthType` + constantes, que sí se usan); las sondas `ProbeProxy*` no se llamaban desde ningún sitio. `sni.go` se conserva: lo usa `socks5.go`. |
| **F5.2** | ✅ | Un solo `wails.json`, en la raíz, con `frontend.dir: unitunnel-go/dist` (el directorio que incrusta `//go:embed all:dist`). Eliminado `unitunnel-go/wails.json`, que apuntaba a `../dist`. |
| **F5.3** | ✅ | README raíz reescrito (era el boilerplate de AI Studio, con `GEMINI_API_KEY` y enlaces a ai.studio) y `unitunnel-go/README.md` reescrito: sin Fyne ni Android, con arquitectura, flujo de conexión, modos, tabla de `config.json`, reversibilidad y resolución de problemas. `index.html` ya no se llama “NTLM Diagnostic Hub”. `metadata.json` (metadatos de AI Studio con la capability de Gemini) eliminado. `config.example.json` ya no lleva `vps_enabled`/`vps_server`/`vps_secret`. |
| **F5.4** | 🟡 | `metadata.json` eliminado y `public/` desaparece con él. `.aistudio/**` y `public/assets/aistudio/**` ya figuran como borrados en el índice (no los borré yo; los dejé como estaban). ⏳ Pendiente: decidir `bun.lock` vs `package-lock.json`. El CI usa `npm ci`, pero el lockfile npm está desincronizado y no se puede regenerar en este entorno. |
| **F5.5** | 🟡 | `pkg/config/config_test.go`: los defaults pasan `Validate`, 13 casos que debe rechazar, round-trip JSON (credenciales y rutas incluidas) y comprobación de que `config.example.json` no tiene claves huérfanas ni le faltan claves. `pkg/tun/wintun_windows_test.go`: tabla de `cidrToDestMask` (normalización de red, `/0`, `/32`, IPv6 rechazada, basura rechazada). ⏳ Pendiente: parseo de cuota con HTML de muestra, vectores MS-NLMP y `CountingPipe` bajo `-race`. **Los tests no se han ejecutado.** |
| **F5.6** | 🟡 | CI con `go vet ./...`, `go build ./...`, `go test ./...`, `npm run lint`, `npm run build`, y comprobaciones explícitas de que existen `go.sum` y `unitunnel-go/dist/index.html`. ⏳ Pendiente: pasar a `go test -race ./...` cuando haya tests de concurrencia. La “generación de bindings” del plan original no aplica: el puente usa `Call.ByName` y no genera ficheros. |
| **F5.7** | ✅ | **Hotspot reescrito sobre Wi-Fi Direct.** El código anterior llamaba a `netsh wlan start hostednetwork`, que en Windows 11 con drivers modernos es inservible: `netsh wlan show drivers` responde `Hosted network supported: No` y `show hostednetwork` queda en `Status: Not available` (el `set mode=allow` sí graba el SSID, por eso parecía que "no hacía nada"). Ahora `pkg/network/hotspot_windows.ps1` (embebido con `//go:embed`, ejecutado con `powershell -File` sobre un temporal con BOM UTF-8) usa `NetworkOperatorTetheringManager` / SoftAP, que es lo que usa Configuración → Punto de acceso móvil y **no consulta NCSI**, así que arranca aunque el equipo esté detrás del proxy UCI sin salida validada. Selección de interfaz: cable (`802.3`) primero, luego Wi-Fi; el perfil se resuelve con `Get-NetConnectionProfile` y, como respaldo, por el GUID del adaptador. **No se escribe nada en la configuración de red**: se respeta el SSID/contraseña que el usuario ya tiene en Windows, y toda ruta de error vuelve a detener el anclaje (rollback). `IsHotspotForced` ahora pregunta a Windows en vez de fiarse de la caché, así el interruptor es correcto tras reiniciar o si el hotspot se tocó desde Configuración. Verificado en vivo en el equipo del usuario: `start` → `STATE=On` + `192.168.137.1/24`, `stop` → `STATE=Off`, `start`/`stop` repetidos son idempotentes y la red quedó intacta. |
| **F5.8** | ✅ | **Icono de ventana y de bandeja arreglado** (2026-10-30). Diagnóstico en el código fuente de Wails v3.0.0-alpha.8 (`application_options.go`, `webview_window_windows.go`, `systemtray.go`, `w32/icon.go`): la ventana de Windows solo usa `Options.Icon` si el recurso de icono **ID 3** no existe (en `unitunnel.rc` el grupo es el ID 100, así que siempre se caía al fallback → `CreateLargeHIconFromImage(nil)` → nada que pintar → icono genérico del SO). El tray, al no encontrar el recurso, cae a los iconos por defecto de Wails y solo aplica el del tray si `tray.SetIcon` se llama antes de `app.Run()` (el impl se crea dentro de `Run()`). Fix aplicado: `Icon: assets.IconPNG` en `application.Options` y `tray.SetIcon(assets.IconPNG)` en `main.go`. Se usa **PNG** (no ICO): Wails valida el formato y pasa los bytes a `CreateIconFromResourceEx`, que en alpha.8 acepta PNG y no ICO de forma fiable (y el tray usa `lo.Must`, que panics si falla → un ICO inválido habría crasheado al arrancar). El paquete `unitunnel-go/assets` (con `IconPNG`/`IconICO` ya embebidos) vuelve a ser código vivo. |

### Bloqueos del entorno (no resolubles desde aquí)

1. **`go.sum` ausente** → sin red hacia `proxy.golang.org` (403) y sin `.zip` en la caché de módulos.
2. **`package-lock.json` desincronizado** → se añadió `@wailsio/runtime@3.0.0-alpha.29` a `package.json`; `npm` falla con `EACCES` al conectar con el registry, así que hay que ejecutar `npm install` una vez y commitear el lockfile (si no, `npm ci` falla en CI y en local).
3. **Toolchain Go local 1.24.2** y `unitunnel-go/go.mod` fijado en `go 1.22.4` (el mínimo que exige Wails v3.0.0-alpha.8). El árbol de trabajo llegó a tener `go 1.26.5`, que la toolchain instalada no puede usar: se revirtió. Aun así, `go build`/`go vet` no se pueden ejecutar aquí porque la descarga de dependencias está bloqueada.
4. **`go.sum` y `package-lock.json`**: el CI falla *a propósito* si faltan (no hay `go mod tidy` ni `npm install` silenciosos). Son los dos únicos pasos que requieren red y los debe ejecutar el usuario una vez.
5. **Verificación realizada**: solo `gofmt -e -l .` (exit 0 → todo el módulo parsea; antes ni eso por el backtick de `hotspot_windows.go`) y revisión estática del frontend (no hay `node_modules`, así que `tsc --noEmit` no se puede ejecutar). El resto es revisión estática contra el código de Wails v3.0.0-alpha.8. **Nada está compilado ni probado en ejecución.**

### Pendiente conocido tras esta ronda

- **F0.1**: generar `go.sum` con `go mod tidy` en una máquina con red y commitearlo. Es lo único que bloquea compilar, `go vet` y ejecutar los tests nuevos.
- **`package-lock.json`**: ejecutar `npm install` y commitear el lockfile (el CI usa `npm ci`).
- **F2.5 (resto)**: cifrado DPAPI de las contraseñas de `config.json`.
- **F5.4**: decidir si se queda `bun.lock` o `package-lock.json` (hoy conviven).
- **F5.5 (resto)**: tests de parseo de cuota, vectores MS-NLMP y `CountingPipe` con `-race`.
- Validación manual en red UCI real (todo el bloque de pruebas de la Fase 5).
- Borrar `unitunnel-go/pkg/tun/diagnostics_other.go` (stub no-Windows con acentos corruptos; el plan ya lo marcaba como a borrar).
- Confirmar que Wails v3 marshalea `SetLabel`/`SetChecked` de la bandeja al hilo de UI (F3.3).

### Nota sobre ficheros de build

En una ronda intermedia, `git status` mostró `.github/workflows/build-windows.yml` y `unitunnel-go/build_windows.bat` como **borrados en el índice** (junto a `.aistudio/**`, ajenos a esta sesión). Se restauraron con `git restore --staged --worktree` y se volvieron a aplicar las mejoras, que la restauración había descartado:

- **Workflow**: `go-version-file: unitunnel-go/go.mod` (antes `1.27.3` hardcodeado), `npm run lint` antes del build, verificación de `unitunnel-go/dist/index.html`, `go mod download` en vez de `go mod tidy`, `go vet ./...` + `go build ./...` como paso de verificación, y eliminada la copia `..\dist` → `unitunnel-go\dist` (Vite ya escribe ahí). También se corrigió el `LEEME.txt` generado, que arrastraba mojibake (`mActricas`, `automA?ticamente`).
- **`build_windows.bat`**: compila el frontend primero (falla si no hay `node_modules` o no se genera `dist/index.html`), `go mod download` + `go vet ./...` antes de compilar, salida a `dist-release\` (igual que el CI) en vez de `..\dist\`, y `wintun.dll`/`tun2socks.exe` se descargan directamente en `dist-release`. Sin acentos (cmd.exe usa la codepage OEM) y con CRLF.

---

## Fase 0 — Baseline reproducible (bloquea todo lo demás)

Sin esto no se puede compilar, ni testear, ni verificar ningún fix.

| ID | Tarea | Archivos | Cambio concreto | Criterio de aceptación | Est. |
|---|---|---|---|---|---|
| **F0.1** | Restaurar `go.sum` | `unitunnel-go/go.sum` | `go mod tidy` con el toolchain correcto y commit del `go.sum`. Verificar que `go.mod` quede con los indirectos reales y sin `golang.org/x/crypto` ni `golang.org/x/net` (no se importan en ningún `.go`). | `go build ./...` y `go vet ./...` sin errores "missing go.sum entry" | 2 h |
| **F0.2** | Fijar toolchain y versión Wails | `unitunnel-go/go.mod`, `.github/workflows/build-windows.yml` | `go 1.22.4` (mínimo que exige `wails v3.0.0-alpha.8`), y CI con la **misma** versión vía `go-version-file`. **No** subir a `v3.0.0` en este PR: exige Go ≥ 1.26 y cambia API de bindings — va en un PR aparte si se decide. | `go.mod` y CI declaran la misma versión; build local y CI idénticos | 2 h |
| **F0.3** | Arreglar el `embed` del frontend | `vite.config.ts`, `unitunnel-go/main.go:22`, `unitunnel-go/build_windows.bat` | Una sola ruta: Vite escribe en `unitunnel-go/dist` (lo que incrusta `//go:embed all:dist`) y el release va a `dist-release/` junto a `wintun.dll`/`tun2socks.exe`/`config.json`, igual que CI. Así el `.exe` nunca se incrusta a sí mismo. `.gitkeep` versionado para que un clone limpio compile. | `npm run build` + `go build` en limpio produce un `.exe` que arranca | 3 h |
| **F0.4** | Limpiar referencias muertas del script | `build_windows.bat`, `unitunnel-go/README.md:38` | Quitar `cmd\unitunnel` (no existe); usar `unitunnel.rc` de la raíz del módulo. | El `.bat` corre de punta a punta en una máquina limpia | 1 h |
| **F0.5** | `go vet` + `npm run lint` en CI | `.github/workflows/build-windows.yml` | Añadir pasos de vet/lint y `npm ci` reproducible. Quitar `go mod tidy` del build (muta el repo). | CI falla si vet o lint fallan | 2 h |

> ⚠️ Verificación pendiente: en el entorno de auditoría las descargas de módulos estaban bloqueadas (403 en `proxy.golang.org`), así que F0.1–F0.2 hay que ejecutarlas en una máquina con acceso al proxy Go.

---

## Fase 1 — Bloqueantes críticos (PR conjunto `fix/critical`)

**F1.1 y F1.2 se mergean juntos**: el puente solo se puede validar con un build que funcione (Fase 0).

| ID | Tarea | Archivos | Cambio concreto | Criterio de aceptación | Est./Riesgo |
|---|---|---|---|---|---|
| **F1.1** | Puente real con Wails v3 | `src/services/wailsBridge.ts`, `package.json`, `vite.config.ts` | 1) Añadir `@wailsio/runtime` con la **misma** versión que la librería Go. 2) Llamar por nombre: `Call.ByName("unitunnel/pkg/service.AppService.<Método>")` (el FQN que emiten los bindings = `PkgPath.Type.Method`), sin generar bindings ni hardcodear IDs. 3) Fallback explícito y **ruidoso**: si no hay runtime, lanzar error visible en UI, nunca silencio. | Con la app empaquetada: cambiar el proxy en Settings persiste en `config.json` del disco; `GetConfig` devuelve datos reales; ningún `localStorage` como fuente de verdad | 8 h / **Alto** (si la versión del runtime JS no coincide con la lib Go, la llamada falla) |
| **F1.2** | Deadlock del hotspot | `unitunnel-go/pkg/service/app_service.go:408-424` | Sacar `s.AddLog` fuera del lock: capturar estado bajo lock, loguear después de `Unlock()`. Aplicar el mismo patrón a cualquier otro método que llame `AddLog` con `s.mu` tomado (auditar `:153-254`, `:274-284`). Alternativa más robusta: hacer el log lock-free con un canal + goroutine consumidora. | Llamar `ToggleForceOfflineHotspot` dos veces seguidas no congela la UI; `GetTrafficStats` sigue respondiendo durante y después | 2 h / Bajo |
| **F1.3** | Eliminar "éxitos" falsos | `wailsBridge.ts:284-302, 442-471` | `saveConfig` y `toggleHotspot` deben devolver el resultado real del backend; borrar el reporte fabricado de "Realtek RTL8821CE … Ready". Si el backend falla, la UI muestra el error. | Con backend caído, la UI muestra error y **no** dice "guardado" | 2 h / Bajo |
| **F1.4** | Crash paths de `main.go` | `unitunnel-go/main.go:40, 110-122` | Nil-check real antes de `win.RegisterHook`; corregir el campo `Time:` que imprime `logPath`. | Arranque sin ventana disponible no paniquea | 1 h / Bajo |

---

## Fase 2 — Seguridad y estado del sistema

Lo más peligroso del proyecto: **la app puede dejar Windows sin internet** y destruir configuración del usuario.

| ID | Tarea | Archivos | Cambio concreto | Criterio de aceptación | Est./Riesgo |
|---|---|---|---|---|---|
| **F2.1** | Snapshot/restore del proxy de Windows | `pkg/sysproxy/proxy_windows.go:23-61, 108-122` | Antes de `EnableSystemProxy`, leer y persistir `ProxyEnable`, `ProxyServer`, `ProxyOverride`, `AutoConfigURL` en un archivo de estado (`%LOCALAPPDATA%\UniTunnel\state.json`). `DisableSystemProxy` restaura **exactamente** esos valores, no `ProxyEnable=0`. | Con un proxy corporativo preexistente: conectar→desconectar deja el registro idéntico al original (comparar export antes/después) | 4 h / **Alto** |
| **F2.2** | Snapshot/restore de métricas de interfaz | `pkg/network/metrics_windows.go:54-107` | Guardar `AutomaticMetric` + `InterfaceMetric` por interfaz antes de tocar; `SetNetworkMode("direct")` restaura los valores guardados, no "automático". | Cambiar de modo ida y vuelta restaura los valores originales | 3 h / Medio |
| **F2.3** | Rutas: no borrar lo ajeno | `pkg/tun/wintun_windows.go:57-72, 304-306` | Registrar cada ruta **añadida por nosotros** (destino/máscara/gw) y borrar solo esas. Nunca `route delete 10.0.0.0` global. | Con una VPN de terceros activa, conectar/desconectar no altera sus rutas | 3 h / **Alto** |
| **F2.4** | Cleanup de arranque | `pkg/tun/diagnostics_windows.go:44-77` (hoy código muerto) | Invocar `PerformStartupDiagnosticsAndClean` desde `main.go` antes de crear la ventana, logueando el `DiagnosticReport`. | Matar el proceso con el túnel activo → al reabrir, internet funciona y el log reporta `CleanedStaleState: true` | 2 h / Medio |
| **F2.5** | Secretos | `pkg/config/config.go:172-210`, `pkg/quota/quota.go:52, 88-106`, `wailsBridge.ts:284-288` | 1) `0600` en `config.json` + escritura atómica (temp + `os.Rename`). 2) Quitar `InsecureSkipVerify`; quitar el fallback `http://` con Basic. 3) No persistir contraseñas en `localStorage` (solo cache en memoria). 4) Ante error de parseo, no caer a defaults silenciosamente: backup del corrupto + aviso en UI. | `config.json` no legible por otros usuarios; `grep -r "password" localStorage` no devuelve nada; config corrupto genera `.bak` y error visible | 4 h / Medio |
| **F2.6** | Logger | `pkg/logger/logger.go:36-53` | No meter `os.Stdout` en el `MultiWriter` cuando el build es `-H windowsgui` (o envolverlo en un writer que ignore errores). Añadir rotación por tamaño y `Close()` real. | Log escrito en `%USERPROFILE%\UniTunnel\unitunnel_debug.log` en build GUI sin consola | 2 h / Bajo |

---

## Fase 3 — Concurrencia y estabilidad

| ID | Tarea | Archivos | Cambio concreto | Criterio de aceptación | Est./Riesgo |
|---|---|---|---|---|---|
| **F3.1** | Cerrar la carrera Start/Stop | `pkg/service/app_service.go:153-223, 226-254` | Máquina de estados (`idle → connecting → connected → stopping`) con `context.CancelFunc` por intento. `StopTunnel` cancela el intento en curso y `StartTunnel` comprueba el token antes de marcar `connected = true`. | `go test -race` sobre un test que lanza Start y Stop concurrentes no reporta carreras y el estado final es coherente | 5 h / **Alto** |
| **F3.2** | Misma carrera en la UI | `src/App.tsx:186-194` | Deshabilitar "Detener" mientras `connecting`, o mapear el click a cancelación explícita. | Doble click rápido no deja el estado inconsistente | 1 h / Bajo |
| **F3.3** | Tray no bloqueante | `main.go:160-187` | `StartTunnel` y `SetNetworkMode` en goroutine (como ya hace el handler de cuota en `:189-197`), con feedback vía eventos. | Abrir menú del tray durante la conexión no congela el menú | 2 h / Bajo |
| **F3.4** | DNS coherente | `pkg/tun/tun2socks.go:91-101`, `pkg/tun/wintun_windows.go:205-217`, `pkg/tunnel/doh.go:109-113` | Si DoH no arranca, **abortar** el túnel y revertir rutas/DNS en vez de seguir. Configurar el DNS del adaptador solo tras confirmar el listener. Usar `cfg.TunDNS` en lugar de `1.1.1.1`/`dns.google` hardcodeados. | Con el puerto 53 ocupado, `StartTunnel` falla con error claro y el sistema queda sin cambios | 4 h / **Alto** |
| **F3.5** | Half-close correcto | `pkg/traffic/counter.go:40-92`, `pkg/tunnel/*.go` (`Pipe`) | Propagar `CloseWrite` en vez de cerrar ambos sockets al primer EOF; esperar ambas direcciones. | Descarga grande que el cliente cierra parcialmente no se trunca | 2 h / Medio |
| **F3.6** | Contadores correctos | `pkg/tunnel/socks5.go:198,220` | Invertir el orden en `CountingPipe` (o renombrar semántica) para que `read_mb` sea descarga. | Test unitario con pipe simulado valida read/write | 1 h / Bajo |
| **F3.7** | Ping sin abrir TCP al proxy | `pkg/service/app_service.go:274-284` | Cachear el RTT medido por el prober periódico en vez de abrir conexión en cada `GetTrafficStats`. | `GetTrafficStats` no genera tráfico de red | 1 h / Bajo |
| **F3.8** | Single-instance | `main.go` | Usar el soporte de single instance de Wails v3 (o mutex con nombre). | Segunda instancia enfoca la primera en vez de fallar al bindear 1080/8081 | 2 h / Bajo |

---

## Fase 4 — Funcionalidad real vs. cosmética

Regla: **cada ajuste de la UI o funciona o se elimina**. Nada de toggles decorativos.

| ID | Setting | Decisión propuesta | Archivos | Est. |
|---|---|---|---|---|
| **F4.1** | `auto_start` | Implementar con clave `Run` en HKCU (y su reverso al desactivar) | nuevo `pkg/autostart`, `SettingsDrawer.tsx` | 2 h |
| **F4.2** | `quota_check_interval_min` | Timer periódico real en `AppService` con `context` cancelable; hoy solo hay un check único a los 1 s (`app_service.go:214-220`) | `app_service.go` | 2 h |
| **F4.3** | `bypass_routes` | Usar `cfg.BypassRoutes` en `wintun_windows.go:224-227` en vez de las 3 rutas hardcodeadas | `wintun_windows.go` | 2 h |
| **F4.4** | `vps_*` | **Eliminar** de UI y config en esta iteración (`tunnel/vps.go` es código muerto y no hay diseño de protocolo). Reintroducir cuando haya especificación | `SettingsDrawer.tsx`, `config.go`, borrar `vps.go` | 1 h |
| **F4.5** | Ruta de config mostrada | Corregir el texto de `SettingsDrawer.tsx:716` (el archivo está junto al `.exe`, `config.go:172-179`) | `SettingsDrawer.tsx` | 15 min |
| **F4.6** | Validación de inputs | Host/puerto/rangos CIDR/URLs validados antes de guardar | `SettingsDrawer.tsx`, `config.go` | 2 h |
| **F4.7** | Cambios que requieren reconexión | Si se editan puertos/cuenta/proxy con túnel activo, avisar y ofrecer reconectar | `App.tsx`, `SettingsDrawer.tsx` | 2 h |
| **F4.8** | Datos personales | Quitar `yoanbrp` de `wailsBridge.ts:90` y `SettingsDrawer.tsx:447`; `defaultConfig` sin cuenta precargada | ambos | 15 min |
| **F4.9** | UI menor | Formatear velocidad (`App.tsx:462`), decidir si el modal de diagnóstico se conecta a un botón real o se borra (`App.tsx:66,498` — hoy inalcanzable), eliminar estado `interfaces` no usado (`App.tsx:40,86`) | `App.tsx` | 2 h |
| **F4.10** | Proxy HTTP en claro | `tunnel/http_proxy.go:107-126` convierte `GET http://…` en `CONNECT host:80`, que Squid deniega. Reenviar la petición con URI absoluta | `http_proxy.go` | 3 h |

---

## Fase 5 — Limpieza, docs, CI y tests

| ID | Tarea | Archivos | Cambio | Est. |
|---|---|---|---|---|
| **F5.1** | Borrar código muerto | `pkg/tunnel/engine.go`, `vps.go`, `audit.go`, `pkg/auth/detector.go`, `pkg/tun/prober.go`, `pkg/tun/diagnostics_other.go` (si aplica), `traffic.StartSpeedMonitor`, `assets/` | Eliminar o conectar. `engine.go` además tiene firma de `NewEngine` distinta a la usada: es una trampa para el próximo que lo lea | 2 h |
| **F5.2** | Unificar `wails.json` | raíz y `unitunnel-go/wails.json` | Uno solo, con `frontend.dir` correcto | 30 min |
| **F5.3** | Docs | `README.md` (raíz), `unitunnel-go/README.md`, `metadata.json` | Reescribir: Wails v3, Windows-only, requisitos (`tun2socks.exe`, Wintun, admin), build, arquitectura. Borrar boilerplate AI Studio/Gemini/Fyne/Android | 4 h |
| **F5.4** | Repo hygiene | `.aistudio/`, `public/assets/aistudio/`, `bun.lock` o `package-lock.json` (elegir uno) | Eliminar artefactos de AI Studio no referenciados | 30 min |
| **F5.5** | Tests (2 archivos ya creados: `config_test.go`, `wintun_windows_test.go`) | nuevos | Prioridad: (a) parseo de config + round-trip ✅, (b) parseo de quota con HTML de muestra, (c) máquina de estados Start/Stop con `-race`, (d) `auth/ntlm.go` vectores MS-NLMP, (e) `CountingPipe` | 10 h |
| **F5.6** | CI completo | workflow | `go vet`, `go test -race ./...`, `npm run lint`, `npm run build`, generación de bindings verificada (falla si `git diff` no está limpio tras generarlos) | 3 h |

---

## Orden de PRs

| PR | Contenido | Justificación |
|---|---|---|
| **PR-1** `fix/critical` | Fase 0 + F1.1–F1.4 | Sin build no se valida nada; sin puente la app es una maqueta. Es el único PR que cambia la naturaleza del producto |
| **PR-2** `fix/system-safety` | F2.1–F2.6 | Evita daño real a la máquina del usuario |
| **PR-3** `fix/concurrency` | F3.1–F3.8 | Estabilidad bajo uso real |
| **PR-4** `feat/real-settings` | F4.1–F4.10 | Honestidad funcional de la UI |
| **PR-5** `chore/cleanup-docs-tests` | F5.1–F5.6 | Cierre |

---

## Checklist de verificación (por PR, antes de merge)

```powershell
# Backend
cd unitunnel-go
go vet ./...
go build ./...
go test -race ./...

# Frontend
cd ..
npm ci
npm run lint
npm run build   # escribe en unitunnel-go/dist (lo incrusta main.go)

# Manual (máquina Windows con red UCI)
# 1. Exportar HKLM/HKCU proxy + métricas ANTES
# 2. Conectar → verificar navegación real
# 3. Toggle hotspot x2 → UI no se congela
# 4. Matar proceso (Task Manager) → reabrir → internet OK
# 5. Desconectar → comparar registro con el export del paso 1
```

---

## Riesgos y rollback

| Riesgo | Mitigación |
|---|---|
| Cambios de registro/rutas dejan la máquina sin red | Snapshot previo obligatorio (F2.1–F2.3) + F2.4 como red de seguridad; probar en VM primero |
| Bindings generados no coinciden con la versión de la lib | Pinnear CLI y lib al mismo tag; verificar en CI con `git diff --exit-code` |
| Actualizar a `wails v3.0.0` rompe la API | PR separado y posterior; hoy exige Go ≥ 1.26 |
| Refactor de `StartTunnel` introduce regresiones | Tests con `-race` **antes** del refactor (F5.5 parcialmente adelantado) |

Rollback: cada PR es independiente y revertible con `git revert`. El único irreversible es el estado del sistema → por eso F2 va antes que cualquier feature.

---

## Definition of Done

1. `go build`, `go vet`, `go test -race ./...`, `npm run lint`, `npm run build` verdes en CI desde un clone limpio.
2. La UI lee y escribe `config.json` real; cero fallbacks silenciosos y cero datos fabricados.
3. Conectar/desconectar/matar el proceso deja el sistema **byte-idéntico** al estado previo (proxy, métricas, rutas, DNS).
4. Ningún toggle de Settings es decorativo: o funciona o no existe.
5. `grep -r "cmd\\\\unitunnel\|fyne\|GEMINI\|yoanbrp"` no devuelve nada.
6. Al menos los 5 grupos de tests de F5.5 en verde.
