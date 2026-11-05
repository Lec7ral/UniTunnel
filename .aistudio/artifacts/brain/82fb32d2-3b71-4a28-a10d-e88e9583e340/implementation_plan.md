# UniTunnel — Port para Android con Wails v3

Plan de arquitectura y hoja de ruta para Android enfocado en cuatro capacidades fundamentales:
1. **Tunelización transparente** (`android.net.VpnService` con autenticación NTLMv2/Squid).
2. **Sistema Multicuentas completo** (almacenamiento seguro, cambio rápido de usuario y consulta por cuenta).
3. **Métricas de red en tiempo real** (velocímetro reactivo, latencia/ping, gráfica histórica con escala dinámica y estado de interfaces).
4. **Consulta y monitoreo de Cuota UCI** (`cuota.uci.cu` con alertas de sobregiro y nivel de navegación).

*(Quedan excluidas las características de escritorio irrelevantes como Hotspot Wi-Fi Direct y modificaciones al registro de Windows).*

---

## Decisiones Críticas & Confirmadas por el Usuario

> [!IMPORTANT]
> **Alcance Requerido:**
> - **Multicuentas:** Mantener la capacidad total de agregar, editar, eliminar y alternar entre múltiples credenciales universitarias directamente desde la pantalla principal, sincronizando la cuota con la cuenta activa.
> - **Métricas de Red:** Conservar el velocímetro en vivo (`formatSpeed`), la medición periódica de RTT/Ping hacia el Squid (`10.0.0.1`), la gráfica de barras con techo dinámico adaptativo y la detección de interfaz activa (Wi-Fi, Datos Móviles o Ethernet USB-C).
> - **Tunelización VpnService:** Enrutamiento transparente a nivel de sistema sin requerir privilegios de root, evitando bucles de conexión mediante `vpnService.protect()`.

---

## 1. Visión General & Capacidades Principales

### A. Tunelización Transparente (Android VPN)
- Intercepta el tráfico TCP/UDP del dispositivo a través de `VpnService` y lo canaliza hacia el motor `tun2socks` en Go.
- Negocia automáticamente la autenticación NTLMv2 con el proxy Squid de la universidad (`10.0.0.1:8080`) sin que las aplicaciones del móvil requieran configuración manual.

### B. Sistema Multicuentas Integral
- **Gestión de Cuentas:** Lista configurable de credenciales universitarias (`username`, `domain`, `password`, `name`).
- **Selector Rápido en Pantalla Principal:** Menú desplegable táctil para cambiar la cuenta activa en un toque.
- **Persistencia Segura:** Guardado local seguro del fichero de configuración (`config.json`).
- **Sincronización Inmediata:** Al cambiar de cuenta, el túnel actualiza las credenciales de autenticación y refresca automáticamente la cuota correspondiente a ese usuario.

### C. Métricas de Red & Telemetría en Vivo
- **Velocímetro en Tiempo Real:** Cálculo instantáneo de velocidad de descarga y subida en KB/s o MB/s.
- **Gráfica de Tráfico con Techo Dinámico:** Historial de 16 muestras con barras reactivas que ajustan su escala proporcionalmente al consumo reciente (evitando barras vacías en tráfico bajo).
- **Latencia / Ping Probe:** Medición periódica de RTT en milisegundos hacia el servidor proxy para informar de la calidad del enlace.
- **Detección de Interfaces Móviles:** Identificación del medio de conexión activo (Wi-Fi de la facultad, conexión de datos móviles o adaptador Ethernet USB-C).

### D. Inspector y Monitor de Cuota UCI
- Lectura y parseo del portal `https://cuota.uci.cu/`.
- Cálculo de megabytes consumidos, cuota asignada, porcentaje de uso y detección de nivel (`Nacional`, `Internet`).
- Detección precisa de estado de **Sobregiro** con indicador fucsia y megabytes excedentes.

---

## 2. Experiencia de Usuario & Diseño Visual en Android

```
┌────────────────────────────────────────────────────────┐
│  ● UniTunnel                           [Ajustes] [Logs]│
│  ────────────────────────────────────────────────────  │
│                                                        │
│                    ┌───────────┐                       │
│                    │     ⏻     │                       │
│                    │ CONECTADO │                       │
│                    └───────────┘                       │
│              ● Túnel Seguro Activo · 24ms              │
│                                                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │ 👤 Juan Pérez (Activa)           [ 3 cuentas ▾ ] │  │
│  │    juan.perez@uci.cu                             │  │
│  └──────────────────────────────────────────────────┘  │
│                                                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │ 🎛 Cuota UCI [Internet]      340.2 MB / 1000.0 MB │  │
│  │ [████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░] 34%   │  │
│  └──────────────────────────────────────────────────┘  │
│                                                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │ 📶 Tráfico de Red [Techo: 1.25 MB/s]    450 KB/s │  │
│  │ ▅▆▇█▇▆▅▄▃▂ ▂▃▄▅                                  │  │
│  └──────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────┘
```

### Adaptaciones Visuales Móviles
- **Cabecera Simplificada:** Sin controles de ventana de escritorio (minimizar/cerrar), respetando el notch superior y la barra de navegación de gestos inferior (`safe-area-inset`).
- **Selector de Cuentas Táctil:** Menú inferior o popover accesible con una sola mano para alternar cuentas sin tener que abrir el panel completo de Ajustes.
- **Cajones Deslizables (Drawers):** Ajustes y Visor de Logs con soporte táctil fluido.

---

## 3. Arquitectura Técnica en Android

### Diagrama de Flujo y Componentes

```
┌─────────────────────────────────────────────────────────────────┐
│                    Aplicación Android (Wails v3)                │
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                 Frontend React + Tailwind                 │  │
│  │  - Dashboard con Botón de Conexión y Ping                 │  │
│  │  - Selector y Gestor Multicuentas (CRUD completo)         │  │
│  │  - Métricas de Tráfico (Velocidad + Sparkline Dinámico)   │  │
│  │  - Barra de Cuota UCI y Visualizador de Sobregiro         │  │
│  └─────────────────────────────┬─────────────────────────────┘  │
│                                │ @wailsio/runtime               │
│                                ▼                                │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    libwails.so (Go Core)                  │  │
│  │                                                           │  │
│  │   ┌────────────────────┐         ┌─────────────────────┐  │  │
│  │   │     AppService     │         │  Traffic Monitor    │  │  │
│  │   │  - Get/SaveConfig  │◄────────┤  - Ping Prober      │  │  │
│  │   │  - Multicuentas    │         │  - Speed & MBs      │  │  │
│  │   │  - Start/StopTunnel│         └─────────────────────┘  │  │
│  │   └──────────┬─────────┘                                  │  │
│  │              │                                            │  │
│  │              ├───────────────────┐                        │  │
│  │              ▼                   ▼                        │  │
│  │   ┌────────────────────┐   ┌───────────────────────────┐  │  │
│  │   │    Quota Client    │   │      tun2socks Engine     │  │  │
│  │   │  - Auth en cuota   │   │  - Lee paquetes del TUN   │  │  │
│  │   │  - Parser HTML/JSON│   │  - SOCKS5 / HTTP Proxy    │  │  │
│  │   └──────────┬─────────┘   └─────────────┬─────────────┘  │  │
│  │              │                           │                │  │
│  │              │     ┌─────────────────────┴─────────────┐  │  │
│  │              │     │ NTLMv2 Tunnel Connector (Squid)   │  │  │
│  │              │     └─────────────────────┬─────────────┘  │  │
│  └──────────────┼───────────────────────────┼────────────────┘  │
│                 │                           │                   │
│                 │     ┌─────────────────────▼─────────────┐     │
│                 │     │ android.net.VpnService (Java/Kt)  │     │
│                 │     │ - Descriptor TUN virtual          │     │
│                 │     │ - vpnService.protect(socketFd)    │     │
│                 │     └─────────────────────┬─────────────┘     │
└─────────────────┼───────────────────────────┼───────────────────┘
                  │                           │
                  ▼                           ▼
      ┌────────────────────────────────────────────────────────┐
      │            Proxy Squid UCI (10.0.0.1:8080)             │
      └───────────────────────────┬────────────────────────────┘
                                  │
                                  ▼
                               Internet
```

---

## 4. Estrategia de Implementación por Fases

### Fase 1: Desacoplamiento de Sistema en Go (`//go:build android`)
- Separar en módulos específicos de compilación las funciones que tocan el Registro de Windows, elevación de privilegios UAC y comandos PowerShell de escritorio.
- Adaptar `TrafficCounter` y `TrafficStats` en Go para reportar velocidad y RTT tanto en Windows como en Android sin alterar el contrato con la interfaz.
- Asegurar que la estructura de cuentas (`accounts: UserAccount[]`) se guarde y cargue fielmente en el almacenamiento privado de la app en Android.

### Fase 2: Integración de `VpnService` y Loopback Protection
- Crear el servicio Android `UniTunnelVpnService` que solicita la autorización del usuario y expone el descriptor TUN al backend Go.
- Implementar la función de protección de sockets (`protect`) para las conexiones salientes hacia el Squid universitario (`10.0.0.1:8080`), previniendo bucles de enrutamiento.

### Fase 3: Optimización de la Interfaz React para Móvil
- Condicionar la cabecera: ocultar los botones de minimizar/cerrar cuando se ejecuta en Android.
- Mantener y verificar el funcionamiento íntegro de:
  - Selector y gestor multicuentas (añadir, editar, alternar y borrar).
  - Velocímetro y gráfica sparkline con techo adaptativo.
  - Medidor de cuota UCI y soporte de sobregiro.
  - Cajón de logs de diagnóstico y cajón de ajustes.

### Fase 4: Compilación y Verificación
- Configuración de la compilación multiplataforma en Wails v3 para arquitecturas móviles (`arm64-v8a`, `x86_64`).
- Verificación de la conexión en un solo toque, cambio dinámico de cuenta y actualización de métricas en tiempo real.
