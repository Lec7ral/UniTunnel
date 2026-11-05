# UniTunnel — Port para Android con Wails v3

Cliente de túnel y acceso transparente a redes universitarias con proxy Squid autenticado (NTLMv2) para dispositivos **Android** (API 26+ / Android 8.0 a 15+).

Construido sobre **Wails v3** (Go backend compilado como `libwails.so` con CGO + Android NDK) y el frontend en **React + Tailwind CSS** ejecutado en `WebView` acelerado por hardware.

---

## Características en Android

1. **Captura de Red Completa sin Root (`VpnService`):**
   - Utiliza la API nativa `android.net.VpnService` de Android.
   - Crea un adaptador virtual TUN (`10.254.1.2/24`) y enruta todo el tráfico de las aplicaciones del móvil (WhatsApp, Telegram, navegadores, GitHub, Play Store).
   - Protección de bucles (`protectSocket`) para que el tráfico saliente hacia el proxy Squid (`10.0.0.1:8080`) no recircule por la interfaz VPN.

2. **Sistema Multicuentas Integral:**
   - Permite registrar, editar y almacenar múltiples cuentas universitarias.
   - Selector rápido desde la pantalla principal para cambiar de cuenta activa al instante.
   - Actualización inmediata de cuota al cambiar de usuario.

3. **Métricas de Red & Telemetría en Tiempo Real:**
   - Velocímetro en vivo (KB/s y MB/s) con contadores de descarga y subida.
   - Gráfica sparkline dinámica con escala adaptativa según el consumo reciente.
   - Medición periódica de latencia / ping hacia el Squid universitario.
   - Detección de interfaces activas (Wi-Fi de la facultad, datos móviles o Ethernet USB-C).

4. **Monitor de Cuota UCI (`cuota.uci.cu`):**
   - Consulta periódica y manual del consumo de navegación.
   - Indicador visual de megabytes restantes / asignados.
   - Alerta en fucsia ante estados de sobregiro.

5. **Segundo Plano Continuo (`ForegroundService`):**
   - Servicio en primer plano con notificación en la barra de estado para evitar que el optimizador de batería de Android detenga el túnel cuando la pantalla se bloquea.

---

## Requisitos de Compilación

- **Go 1.22.4+** con soporte CGO.
- **Node.js 20+** y `npm`.
- **Java JDK 17 o 21** (`JAVA_HOME`).
- **Android SDK** (Build-Tools, Platform Tools y Platform API 35).
- **Android NDK** (r26 o superior).
- Variable de entorno `ANDROID_HOME` o `ANDROID_SDK_ROOT` configurada.

---

## Instrucciones de Compilación

### 1. Compilar el Frontend React
```bash
npm install
npm run build
```
*(Escribe la salida en `unitunnel-go/dist`, embebida por Wails v3).*

### 2. Compilar con Wails v3 CLI
```bash
# Diagnosticar el entorno de desarrollo Android
wails3 doctor

# Compilar el paquete APK para Android
wails3 task android:build

# O ejecutar directamente en un dispositivo o emulador conectado
wails3 task android:run
```

### 3. Compilación directa con Gradle
```bash
cd android
./gradlew assembleDebug
```
El APK resultante se genera en:
`android/app/build/outputs/apk/debug/app-debug.apk`

---

## Estructura del Port Android

```
android/
├── AndroidManifest.xml
├── build.gradle
└── app/
    ├── build.gradle
    └── src/main/
        ├── AndroidManifest.xml
        ├── res/xml/network_security_config.xml
        └── java/cu/uci/unitunnel/
            ├── MainActivity.kt          # Host WebView Wails y solicitud de permiso VPN
            └── UniTunnelVpnService.kt   # Implementación del VpnService y protectSocket
```
