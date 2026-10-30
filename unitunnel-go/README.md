# UniTunnel — backend Go (Wails v3)

Backend del cliente de túnel. Expone un `AppService` a la UI React, habla `HTTP CONNECT`
con el proxy de la universidad (NTLM o Basic) y publica ese túnel al sistema operativo
de dos formas: proxy de sistema o adaptador virtual Wintun.

> Solo Windows. Los paquetes con lógica de sistema llevan build tag `_windows.go` y
> tienen un stub `_other.go` para que `go vet ./...` funcione en cualquier plataforma.

---

## Compilar

```cmd
cd ..
npm ci
npm run build          :: escribe en unitunnel-go\dist (lo que incrusta el .exe)
cd unitunnel-go
go mod download
go vet ./...
go build -ldflags="-H windowsgui -s -w" -o ..\dist-release\unitunnel.exe .
```

`build_windows.bat` automatiza todo (frontend, `go vet`, `wintun.dll`, `tun2socks.exe`)
y deja el paquete en `dist-release\`.

Requisitos: Go 1.22.4+, MinGW-w64 (cgo) y permisos de Administrador para ejecutar.

---

## Arquitectura

| Paquete | Responsabilidad |
|---|---|
| `main.go` | Arranque Wails, instancia única, autostart, diagnóstico/limpieza inicial, recuperación de pánicos. |
| `pkg/service` | `AppService`: máquina de estados `idle → connecting → connected → stopping`, cuota, ping, logs. Es lo único que ve el frontend. |
| `pkg/tunnel` | `Dialer` (CONNECT + NTLM/Basic), servidor SOCKS5, proxy HTTP, DoH local, sniffer SNI / DNS inverso. |
| `pkg/tun` | Adaptador Wintun, lanzamiento de `tun2socks`, rutas y su libro de propiedad (`routes_state.json`). |
| `pkg/sysproxy` | Proxy del sistema en el registro, con snapshot/restore. |
| `pkg/network` | Métricas de interfaz (prioridad Cable/Wi-Fi), hotspot Wi-Fi Direct, snapshot de métricas. |
| `pkg/quota` | Consulta del portal de cuota (HTML → `QuotaInfo`). |
| `pkg/config` | `config.json` atómico, validación, backup de ficheros corruptos. |
| `pkg/appstate` | Estado persistente en `%LOCALAPPDATA%\UniTunnel`. |
| `pkg/logger` | Log a fichero con rotación; nunca tumba la app si stdout no existe. |
| `pkg/singleinstance`, `pkg/autostart`, `pkg/elevation`, `pkg/auth` | Integración con Windows y primitivas NTLM. |

### Flujo de una conexión

1. `StartTunnel` valida el estado (`connState`) y crea el `Engine`.
2. Se abren los listeners locales: SOCKS5 (`local_socks_port`, 1080) y HTTP (`local_http_port`, 8081).
3. Se arranca el resolver DoH en UDP `127.0.0.1:53`, que reenvía a los resolvers de
   `tun_dns` (`https://<ip>/dns-query`).
4. Según `capture_mode`:
   - `system_proxy`: se escribe el proxy de Windows apuntando a `127.0.0.1:8081`.
   - `tun`: se crea el adaptador Wintun, se le asigna `tun_ipv4`/`tun_mask`, DNS
     `127.0.0.1`, y se lanzan las rutas `0.0.0.0/1` + `128.0.0.0/1` hacia `tun2socks`.
5. Al desconectar se revierte todo en orden inverso, usando únicamente el estado
   guardado (proxy, métricas, rutas).

---

## Modos

### Captura (`capture_mode`)

| Valor | Qué hace | Requiere admin |
|---|---|---|
| `system_proxy` | Cambia el proxy de Windows. Solo cubre apps que respetan el proxy del sistema. | No (recomendado si no hay permisos) |
| `tun` | Todo el tráfico IPv4 del equipo pasa por el túnel. | Sí |

### Red (`network_mode`)

| Valor | Efecto |
|---|---|
| `uci_cable` | Túnel activo; Ethernet con métrica 10, Wi-Fi 100. |
| `wifi_mobile` | Wi-Fi con métrica 10, Ethernet 100; permite usar el hotspot del móvil con el cable conectado. |
| `direct` | Restaura las métricas automáticas de Windows y no aplica proxy. |

---

## `config.json`

Se guarda **junto a `unitunnel.exe`**, con escritura atómica y permisos restringidos.
Plantilla: `config.example.json`.

| Clave | Descripción |
|---|---|
| `proxy_host`, `proxy_port` | Proxy Squid institucional (por defecto `10.0.0.1:8080`). |
| `auth_type` | `auto`, `ntlm`, `basic` o `none`. `auto` negocia NTLM. |
| `accounts[]`, `active_account_id` | Perfiles de cuenta; el activo es el que se usa. |
| `local_socks_port`, `local_http_port` | Puertos locales del túnel (deben ser distintos). |
| `capture_mode` | `tun` o `system_proxy`. |
| `tun_name`, `tun_ipv4`, `tun_mask` | Identidad del adaptador Wintun. |
| `tun_dns` | Lista separada por comas de resolvers DoH. |
| `bypass_routes` | CIDRs que **no** pasan por el túnel. |
| `network_mode` | Perfil de red (`uci_cable`, `wifi_mobile`, `direct`). |
| `quota_url` | Portal de cuota. Debe ser `https://` si hay credenciales. |
| `auto_check_quota`, `quota_check_interval_min` | Consulta periódica de cuota. |
| `start_minimized`, `auto_start`, `always_on_top` | Integración con Windows. |

Cambiar proxy, puertos, cuenta, modo de captura, Wintun o rutas **requiere reconectar**:
el túnel se construye una sola vez con esos valores. La UI avisa de ello.

---

## Seguridad y reversibilidad

El objetivo es que un cierre inesperado no deje el equipo sin red.

- **Proxy del sistema**: antes de escribir nada se guarda `ProxyEnable`, `ProxyServer`,
  `ProxyOverride`, `AutoConfigURL` y las variables `*_proxy` de `HKCU\Environment` en
  `%LOCALAPPDATA%\UniTunnel\proxy_state.json`. Si el snapshot falla, la operación se
  aborta. Al desconectar se restaura valor por valor (incluido borrar lo que no existía).
- **Métricas de interfaz**: snapshot por interfaz antes del primer cambio; `direct`
  restaura los valores exactos.
- **Rutas**: solo se borran las que UniTunnel añadió (`routes_state.json`). Nunca se
  ejecuta un `route delete` global.
- **Arranque**: `main.go` ejecuta la limpieza de estado huérfano y la restauración de
  métricas antes de aplicar el modo guardado.
- **Cuota**: sin `InsecureSkipVerify` y sin degradar a HTTP con Basic.
- **Instancia única**: un mutex con nombre evita dos procesos peleando por los puertos
  1080/8081 y por las claves del registro.

---

## Resolución de problemas

| Síntoma | Causa probable |
|---|---|
| Ventana en blanco | Se compiló el `.exe` sin `npm run build`; `unitunnel-go/dist` está vacío. |
| “No se encontró `wintun.dll`” / `tun2socks.exe` | Faltan junto al `.exe`. |
| El adaptador Wintun no arranca | Falta ejecutar como Administrador. |
| `CONNECT` falla solo con IPs numéricas | El ACL del Squid (`numeric_IPs`) exige FQDN; el sniffer SNI intenta sustituirlas. |
| HTTP en claro devuelve 502 | El ACL deniega `CONNECT host:80`; el proxy reenvía con URI absoluta, revisa los logs. |
| Todo el equipo sin red tras cerrar | Reinicia: el arranque restaura proxy y métricas desde `%LOCALAPPDATA%\UniTunnel`. |

Los logs se escriben en `%USERPROFILE%\UniTunnel\unitunnel_debug.log` y junto al `.exe`
(rotación a 5 MB, `.1`), y también se ven en el panel de logs de la UI.
