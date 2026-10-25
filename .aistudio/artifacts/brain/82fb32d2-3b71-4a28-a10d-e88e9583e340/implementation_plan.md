# Rediseño Minimalista: Ventana Compacta de Escritorio (Estilo Fyne Widget)

Plan integral para simplificar y limpiar la interfaz de UniTunnel, adoptando una estructura compacta centrada estilo widget nativo de escritorio (inspirada en la ergonomía y sobriedad de la interfaz de Fyne), eliminando paneles innecesarios y conservando en la vista central únicamente los 4 elementos esenciales solicitados por el usuario.

---

### Decisiones Críticas y Confirmaciones del Usuario

> [!IMPORTANT]
> **Decisiones confirmadas por el usuario**:
> - **Estructura Visual**: Ventana compacta centrada estilo widget de escritorio (~440px - 460px de ancho), eliminando paneles anchos y banners innecesarios.
> - **Contenido Central Exclusivo**:
>   1. Botón principal de **Conexión** con estado claro (Conectado / Desconectado / Negociando).
>   2. Monitor de **Cuota de Navegación** (barra de progreso visual, MB restantes/consumidos y soporte seguro de sobregiro).
>   3. Selector de **Cuenta Activa** (perfil, usuario y dominio).
>   4. Mini-gráfica de **Velocidad y Latencia** en tiempo real.
> - **Ubicación de Modos de Red**: En su propio menú/selector compacto (dropdown elegante integrado en la barra de cabecera de la ventana y sincronizado con la Bandeja del Sistema).
> - **Ajustes y Logs**: Se mantienen accesibles en las esquinas superior izquierda y derecha mediante los cajones laterales deslizables sin sobrecargar la pantalla principal.

---

### 1. Visión General del Producto y Valor

El rediseño transforma UniTunnel de una consola web extendida a una **herramienta de escritorio ultra enfocada y minimalista**, que ocupa el espacio justo en pantalla sin distracciones. Ofrece al usuario una experiencia de control rápido donde el 100% de las tareas diarias (conectar, cambiar de modo de red, revisar cuota y monitorear velocidad) se realizan en una ventana elegante y compacta.

---

### 2. Experiencia de Usuario y Diseño Visual

#### A. Disposición Visual (Ventana Compacta ~450px)
- **Barra de Ventana y Cabecera**:
  - **Izquierda**: Botón discreto de **Ajustes** (engranaje) para abrir el panel lateral completo de `config.json`.
  - **Centro**: Título `"UniTunnel"` con badge sutil del **Modo de Red activo** que actúa como menú desplegable para alternar al instante entre `Cable UCI`, `Wi-Fi / Móvil` y `Directo`.
  - **Derecha**: Botón de **Fijar ventana** (chincheta para mantenerla al frente sobre otras apps) y botón de **Logs** con contador de eventos.
- **Cuerpo Central (Los 4 Elementos Fundamentales)**:
  1. **Indicador de Estado y Botón de Conexión**:
     - Botón de gran tamaño y respuesta inmediata con cambio de color según estado (Verde conectado / Rojo desconectar / Ámbar negociando).
     - Línea de datos del proxy configurado (`10.0.0.1:8080 · Wintun L3`).
  2. **Selector de Cuenta Activa**:
     - Fila compacta que muestra el nombre del perfil (`Cuenta Principal`), usuario (`yoanbrp`) y dominio (`UCI`).
     - Selector desplegable directo si el usuario tiene más de una cuenta configurada.
  3. **Tarjeta de Cuota de Navegación (Resiliente a Sobregiro)**:
     - Barra de progreso con porcentaje acotado (0–100%) para evitar distorsiones de maquetación.
     - Indicador numérico claro: `Restante: X MB / Total: Y MB` o `Sobregiro: +X MB` con badge *"Sobregiro permitido"*.
     - Botón de refrescar cuota en 1 clic.
  4. **Mini-Monitor de Velocidad y Latencia**:
     - Gráfica de barras sparkline en tiempo real (MB/s) y latencia DNS DoH (ms).

#### B. Ergonomía y Estética
- **Paleta de Colores**: Fondo neutro pizarra oscura (`slate-950`), tarjeta principal en `slate-900` con borde sutil `slate-800`, acentos cian/esmeralda para acciones primarias.
- **Sin Elementos Innecesarios**: Se eliminan de la vista principal los diagnósticos de 4 pasos, banners explicativos de NTLMv2 y tarjetas de bienvenida, dejándolos disponibles en el panel de logs y ajustes.

---

### 3. Decisiones de Producto y Arquitectura

- **Decisión 1: Modos de Red en Menú Desplegable Compacto**:
  - *Enfoque*: En lugar de ocupar una fila entera con botones segmentados anchos, se integra como un menú desplegable elegante en la cabecera de la ventana compacta.
  - *Razón*: Ahorra espacio vertical y horizontal, manteniendo el selector accesible en 1 clic tanto en la ventana como en el System Tray de Windows.
- **Decisión 2: Preservación de Cajones Laterales**:
  - *Enfoque*: Los cajones de **Ajustes** (izquierda) y **Logs** (derecha) permanecen intactos, desplegándose solo cuando el usuario los solicita.
  - *Razón*: Mantiene la ventana principal limpia sin perder la capacidad de modificar cualquier parámetro de `config.json` o auditar logs.

---

### 4. Diagrama del Sistema y Componentes

```
┌────────────────────────────────────────────────────────┐
│             Ventana Compacta UniTunnel (~450px)        │
├────────────────────────────────────────────────────────┤
│ [⚙ Ajustes]    [Modo Red ▾: Cable UCI]   [📌] [Logs ≡] │
├────────────────────────────────────────────────────────┤
│                                                        │
│                  ● CONECTADO · 38ms                    │
│                                                        │
│           ┌────────────────────────────────┐           │
│           │      [ ■ DESCONECTAR ]         │           │
│           └────────────────────────────────┘           │
│              Proxy: 10.0.0.1:8080 · L3                 │
│                                                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │ 👤 Cuenta: yoanbrp (UCI)                  [Cambiar]│  │
│  └──────────────────────────────────────────────────┘  │
│                                                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │ ⚡ Cuota UCI: 137.6 MB restantes / 500 MB     [↻]│  │
│  │ [██████████████████████░░░░░░░░] 72%             │  │
│  └──────────────────────────────────────────────────┘  │
│                                                        │
│  ┌──────────────────────────────────────────────────┐  │
│  │ 📶 Tráfico: 2.6 MB/s · 35ms                       │  │
│  │  ▂▃▅▆▇▆▅▄▅▆▇ (Sparkline en tiempo real)          │  │
│  └──────────────────────────────────────────────────┘  │
│                                                        │
└────────────────────────────────────────────────────────┘
```

---

### 5. Plan de Ejecución

1. **Reestructuración de `src/App.tsx`**:
   - Ajustar el contenedor raíz para centrar la ventana compacta con aspecto de aplicación nativa.
   - Diseñar la cabecera con el menú de Modos de Red compacto (`Cable UCI`, `Wi-Fi / Móvil`, `Directo`).
   - Ubicar los 4 bloques centrales: Botón de Conexión, Cuenta Activa, Monitor de Cuota y Gráfica de Velocidad.
   - Conectar los botones de apertura de `SettingsDrawer` (izquierda) y `LogsDrawer` (derecha), y el botón de fijar ventana `always_on_top`.
2. **Validación y Compilación**:
   - Ejecutar `lint_applet` y `compile_applet` para garantizar cero errores de tipos y empaquetado impecable.
