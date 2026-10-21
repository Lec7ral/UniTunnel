# UniTunnel: Native Go + Fyne Desktop Plan & Execution

## Estado de Ejecución

### 1. Limpieza de Repositorio (.gitignore)
- [x] Creado `.gitignore` en la raíz del proyecto para excluir todos los artefactos de desarrollo web (`node_modules/`, `src/`, `package.json`, `dist/`, etc.).
- [x] Asegurado que el repositorio solo mantenga el código nativo de Go (`unitunnel-go/`), sus paquetes, y las plantillas limpias (`config.example.json`), excluyendo credenciales locales y binarios compilados (`*.exe`, `wintun.dll`).

### 2. Interfaz Gráfica (Rollback a la Versión Anterior Solicitada)
- [x] **Restauración de Pestañas Nativas**: Revertido el diseño de cajones colapsables para volver a la estructura de pestañas limpias (`container.NewAppTabs`):
  - **Pestaña Conexión**: Encabezado estilizado, estado de conexión, botón principal `CONECTAR` / `DESCONECTAR`, selector y administración de perfiles de usuario, y gráfico de tráfico en tiempo real `RealtimeChart` en la parte inferior.
  - **Pestaña Ajustes**: Configuración del proxy, puerto, tipo de autenticación, modo de captura (Wintun / Proxy Windows), VPS opcional, botón de prueba "Sondear Proxy" y guardado.
  - **Pestaña Registro**: Consola de eventos y diagnóstico con botones para copiar y limpiar.
- [x] **Geometría Equilibrada**: Dimensiones optimizadas a 420x580 para garantizar espacio holgado sin cortes de texto.
- [x] **Bandeja del Sistema (Tray)**: Minimización automática a la bandeja de Windows con menú contextual e icono incrustado.

### 3. Protección Anti-Bloqueo de Cuenta (Active Directory / Squid) [CRÍTICO]
- [x] **Verificación Preliminar Pre-Flight**: Al pulsar "CONECTAR", UniTunnel realiza una prueba única controlada con el proxy. Si las credenciales fallan, **NO** arranca Wintun, **NO** configura el proxy del sistema y **NO** inicia bucles de reintento.
- [x] **Circuit Breaker Anti-Bloqueo en Dialer**: Ante cualquier rechazo HTTP 407 o `ERR_CACHE_ACCESS_DENIED`, se bloquean instantáneamente todas las peticiones posteriores a nivel local para impedir que el Directorio Activo (AD) de la universidad bloquee la cuenta.
- [x] **Detención Automática Inmediata**: Si se detecta un error de autenticación con el túnel en marcha, se restaura la configuración de red y se notifica al usuario sin repetir peticiones.

### 4. Autenticación NTLMv2 Integral (RFC 1320 MD4)
- [x] **Hash MD4 Autocontenido (RFC 1320)**: Implementación fiel de MD4 para el cálculo exacto del NT-Hash.
- [x] **NTLMv2 HMAC-MD5**: Soporte completo de desafío Type-2, marcas de tiempo Windows FileTime, nonce del cliente y estructura TargetInfo.
- [x] **Auto-Extracción de Dominio**: Soporte automático para formatos `DOMINIO\usuario`, `usuario@dominio` o inferencia desde el desafío enviado por Squid.

### 5. Diagnóstico y Logs Claros
- [x] **Logs Categorizados**: Marcas de tiempo y categorías legibles (`[SISTEMA]`, `[CONFIG]`, `[SONDA]`, `[AUTH-NTLM]`, `[AUTH-OK]`, `[AUTH-FALLO]`, `[SEGURIDAD]`, `[TÚNEL]`).
- [x] **Sonda Limpia**: Diálogo modal informativo con servidor, esquema detectado y diagnóstico, sin volcar cabeceras HTTP irrelevantes.
