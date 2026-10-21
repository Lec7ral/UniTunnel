# UniTunnel: Native Go + Fyne UI Refactoring Plan

## Estado de Ejecución

### 1. Limpieza de Repositorio (.gitignore)
- [x] Creado `.gitignore` en la raíz del proyecto para excluir todos los artefactos de desarrollo web (`node_modules/`, `src/`, `package.json`, `dist/`, etc.).
- [x] Asegurado que el repositorio solo mantenga el código nativo de Go (`unitunnel-go/`), sus paquetes, y las plantillas limpias (`config.example.json`), excluyendo credenciales locales y binarios compilados (`*.exe`, `wintun.dll`).

### 2. Rediseño y Geometría de Ventana Fyne
- [x] **Dimensiones**: Cambiado de 500x420 (horizontal/aplastado) a 365x640 (formato vertical estilizado tipo teléfono/OpenVPN Connect / Cloudflare WARP).
- [x] **Fijación de Tamaño**: `w.SetFixedSize(true)` para evitar distorsiones visuales.

### 3. Paneles Desplegables en Posición (Sin Pantallas Superpuestas)
- [x] **Ajustes de Conexión**: Al presionar el engrane superior izquierdo, se despliega un panel *inline* directamente debajo de la barra superior. Contiene configuración de proxy, puerto, tipo de autenticación, botón de sonda y VPS con botón de guardar y cerrar.
- [x] **Registro en Vivo (Logs)**: Al presionar el documento superior derecho, se despliega el panel de consola en vivo con botones de copiar y limpiar directamente en la ventana.
- [x] Ambas secciones cuentan con cierre y guardado rápido sin abrir ventanas emergentes invasivas (`dialog.NewCustom...`).

### 4. Reubicación del Gráfico de Banda y Latencia al Fondo
- [x] **Posición Inferior**: El `RealtimeChart` ahora está fijado estrictamente en la base de la ventana mediante un layout de frontera (`container.NewBorder`).
- [x] **Diseño Limpio**: Gráfico estilizado con fondo Slate oscuro, línea de flujo esmeralda (MB/s) y línea de latencia celeste (ms), con leyenda explicativa.
- [x] **Sin Solapamientos**: El selector de cuentas universitarias ahora descansa limpiamente en la sección central con su propio contenedor y botones de añadir (+) y eliminar (-) perfiles.
