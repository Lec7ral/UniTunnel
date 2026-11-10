# Modo de Encadenamiento UniTunnel: Proxifier, Psiphon y VPNs Externas

Este plan define la estrategia técnica y de interfaz para permitir la coexistencia y encadenamiento fluido entre **UniTunnel**, **Proxifier** y **Psiphon / VPNs externas**, eliminando los bloqueos por bucles de red en Windows.

---

## User Review & Critical Decisions

> [!IMPORTANT]
> **Decisión confirmada por el usuario:**
> 1. **Flujo de tráfico multietapa:** Integración para canalizar tráfico a través de **UniTunnel** (autenticación NTLMv2 ante Squid UCI `10.0.0.1:8080`), **Psiphon** (túnel SSH/VPN hacia el exterior de Cuba) y **Proxifier** (enrutador selectivo de aplicaciones en Windows).
> 2. **Comportamiento del adaptador Wintun:** En el modo de encadenamiento, **Wintun se desactiva automáticamente**. UniTunnel opera como un *puente proxy local ultraligero* (`127.0.0.1:8081` HTTP y `127.0.0.1:1080` SOCKS5), permitiendo que Proxifier y Psiphon gestionen la captura de sockets sin colisiones de controladores WFP/L3.

- **Puertos locales por defecto:** `127.0.0.1:8081` (HTTP para upstream de Psiphon) y `127.0.0.1:1080` (SOCKS5 para Proxifier).
- **Exportador `.ppx`:** Generación y descarga directa de un perfil XML para Proxifier con reglas automáticas de exclusión (evitando bucles de enrutamiento con `unitunnel.exe` y `10.0.0.1:8080`).

---

## 1. Overview & Core Concept

### El Problema Técnico Actual
Cuando se ejecutan UniTunnel (en modo TUN) y Proxifier simultáneamente:
1. Proxifier (capa 4/7 WFP) intercepta los paquetes generados por `tun2socks.exe` y `unitunnel.exe`.
2. Al reenviarlos de vuelta al túnel o a la misma interfaz, se produce un **bucle infinito de enrutamiento** (routing loop).
3. Ambos programas intentan forzar la pasarela por defecto (Default Gateway) y resolver DNS de formas incompatibles, bloqueando la conexión.

### La Solución: "Modo Encadenamiento" (Chaining Mode)
UniTunnel introduce un nuevo perfil de captura y red:
* **UniTunnel como Puente NTLMv2:** Recibe tráfico no autenticado en `127.0.0.1:8081` / `1080`, le inyecta la autenticación NTLMv2 de la UCI y lo entrega a Squid (`10.0.0.1:8080`).
* **Psiphon como Túnel Exterior:** Se configura con Upstream Proxy hacia UniTunnel (`127.0.0.1:8081`). Al tener credenciales válidas provistas por UniTunnel, Psiphon logra atravesar el Squid de la UCI y conectarse a sus servidores en el extranjero.
* **Proxifier como Enrutador Granular:** Permite al usuario enviar aplicaciones específicas (juegos, Discord, navegador) a través de Psiphon, mientras que las páginas de la universidad (`*.uci.cu`) van directo o a través de UniTunnel.

---

## 2. User Experience & Visual Design

### Nueva Sección: "Modo Encadenamiento" en UniTunnel
1. **Selector de Modo en Pantalla Principal:**
   - Adición del botón de modo `Encadenamiento (Proxifier / Psiphon)` en el selector de perfiles de red.
   - Al seleccionarse, la interfaz informa claramente: *"Adaptador virtual desactivado. Proxies locales listos para encadenar con Proxifier y Psiphon."*

2. **Panel / Drawer de Herramientas de Encadenamiento (`ChainingDrawer.tsx`):**
   - **Diagrama Visual Interactivo:** Representación visual del flujo de datos:
     `Tu Aplicación` ──> `Proxifier` ──> `Psiphon` ──> `UniTunnel (NTLMv2)` ──> `Squid UCI` ──> `Internet`
   - **Datos de Conexión Rápidos:** Botones de "Copiar al portapapeles" para:
     - Proxy HTTP Upstream (para Psiphon): `127.0.0.1:8081`
     - Proxy SOCKS5 (para Proxifier / Navegador): `127.0.0.1:1080`
   - **Botón "Descargar Perfil Proxifier (.ppx)":** Genera un archivo `.ppx` descargable listo para abrir en Proxifier con:
     - Regla Direct: `unitunnel.exe`, `tun2socks.exe`, `10.0.0.1` -> Directo (sin loop).
     - Regla UCI: `*.uci.cu`, `10.*.*.*` -> UniTunnel SOCKS5 (`127.0.0.1:1080`).
     - Regla General: Tráfico externo -> Psiphon SOCKS5 (`127.0.0.1:10808` configurable).
   - **Instrucciones Rápidas Paso a Paso:** Guía visual con los 3 pasos necesarios para conectar Psiphon con UniTunnel.

---

## 3. Key Product Decisions & Trade-Offs

| Decisión | Enfoque Seleccionado | Razón Técnica |
| :--- | :--- | :--- |
| **Coexistencia con Wintun** | Desactivar Wintun en Modo Encadenamiento | Proxifier y Psiphon ya manipulan rutas y sockets a nivel WFP/LSP. Intentar activar un TUN L3 simultáneamente causa colisión de pasarelas y congelamiento de sockets. |
| **Generación de Perfil Proxifier** | Generador de XML `.ppx` estándar | Evita que el usuario tenga que configurar manualmente IPs, puertos y reglas de exclusión complejas donde un error genera bucles. |
| **Monitoreo de Cuota en Chaining Mode** | Se mantiene activo en segundo plano | La consulta a `cuota.uci.cu` sigue funcionando mediante las credenciales activas del usuario a través del socket HTTP local. |

---

## 4. Technical Architecture & Data Strategy

### Diagrama de Flujo del Encadenamiento

```
┌──────────────────────────────────────────────────────────────┐
│                    APLICACIONES DE WINDOWS                   │
│          (Navegadores, Juegos, Telegram, Terminales)         │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│                   PROXIFIER (Controlador WFP)                │
│    Regla UCI (*.uci.cu)          │   Regla General (Internet)│
└──────────────┬───────────────────┼───────────────┬───────────┘
               │                   │               │
               ▼                   │               ▼
┌──────────────────────────────┐   │   ┌───────────────────────┐
│     UniTunnel SOCKS5         │   │   │     PSIPHON CLIENT    │
│     (127.0.0.1:1080)         │   │   │   (SOCKS5 127.0.0.1)  │
└──────────────┬───────────────┘   │   └───────────┬───────────┘
               │                   │               │ Upstream Proxy
               │                   │               ▼
               │                   │   ┌───────────────────────┐
               │                   │   │    UniTunnel HTTP     │
               │                   │   │   (127.0.0.1:8081)    │
               │                   │   └───────────┬───────────┘
               │                   │               │
               ▼                   ▼               ▼
┌──────────────────────────────────────────────────────────────┐
│           UNITUNNEL MOTOR NTLMv2 (pkg/auth/ntlm.go)          │
│       Inyecta credenciales universitarias (Dominio UCI)       │
└──────────────────────────────┬───────────────────────────────┘
                               │ Conexión TCP autenticada
                               ▼
┌──────────────────────────────────────────────────────────────┐
│                PROXY SQUID UCI (10.0.0.1:8080)               │
└──────────────────────────────┬───────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────┐
│             SERVIDORES EXTERNOS PSIPHON / INTERNET           │
└──────────────────────────────────────────────────────────────┘
```

### Cambios a Implementar en el Código

1. **Backend Go (`unitunnel-go/`):**
   * `pkg/config/config.go`: Agregar modo `"chaining"` en `CaptureMode` o `NetworkMode`.
   * `pkg/service/app_service.go`: Manejar la inicialización en modo encadenamiento (arranca solo los servidores proxy SOCKS5 y HTTP con autenticación NTLMv2 sin instanciar Wintun ni tun2socks).
   * `pkg/proxifier/ppx.go`: Nuevo módulo para generar el XML `.ppx` con las reglas de enrutamiento y exclusión para Proxifier.
   * Exponer método en AppService: `ExportProxifierProfile(psiphonPort int) string`.

2. **Frontend React (`src/`):**
   * `src/components/ChainingDrawer.tsx`: Componente interactivo para configurar el encadenamiento, ver el estado de los puertos y descargar el archivo `.ppx`.
   * `src/App.tsx`: Añadir la opción en el selector de modo de red y botón de acceso directo a la configuración de encadenamiento.
   * `src/services/wailsBridge.ts`: Mapear la función de exportación del perfil `.ppx` y descarga en el navegador / Windows.
