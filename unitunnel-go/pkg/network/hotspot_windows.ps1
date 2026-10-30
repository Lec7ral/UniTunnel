# Punto de acceso movil (Wi-Fi Direct / SoftAP) para Windows 10/11.
#
# El API clasico "netsh wlan hostednetwork" esta muerto en los drivers modernos:
# Windows 11 responde "Hosted network supported: No" y se niega a arrancarlo. La
# pantalla Configuracion > Punto de acceso movil usa Wi-Fi Direct, y a diferencia
# de esa pantalla NO exige que NCSI detecte internet, que es exactamente el caso
# cuando el equipo esta detras del proxy de la UCI.
#
# Reglas de seguridad de este script:
#   * NUNCA reescribe la configuracion del punto de acceso. El SSID y la
#     contrasena se quedan tal como el usuario los tiene en Windows, asi que no
#     hay nada que restaurar y nada que pueda quedar roto.
#   * Toda ruta de error vuelve a detener el anclaje, para no dejar el adaptador
#     a medio arrancar.
#   * Codigo de salida 0 = ok, distinto de 0 = error. Los errores se imprimen
#     como "ERROR: ..." por stdout.
#
# Acciones (las inyecta Go antes de este contenido):
#   probe  -> STATE=On|Off y CAPABILITY=Enabled|Disabled
#   status -> informe legible para la UI
#   start  -> arranca el anclaje
#   stop   -> lo detiene y verifica que quedo apagado

$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$OutputEncoding = [System.Text.Encoding]::UTF8

Add-Type -AssemblyName System.Runtime.WindowsRuntime | Out-Null

# Convierte una IAsyncOperation<T> de WinRT en una llamada bloqueante.
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
    $_.Name -eq 'AsTask' -and
    $_.GetParameters().Count -eq 1 -and
    $_.GetParameters()[0].ParameterType.Name.StartsWith('IAsyncOperation')
})[0]

function Await($WinRtTask, $ResultType) {
    $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
    $netTask = $asTask.Invoke($null, @($WinRtTask))
    $netTask.Wait(-1) | Out-Null
    $netTask.Result
}

[Windows.Networking.Connectivity.NetworkInformation, Windows.Networking.Connectivity, ContentType = WindowsRuntime] | Out-Null
[Windows.Networking.NetworkOperators.NetworkOperatorTetheringManager, Windows.Networking.NetworkOperators, ContentType = WindowsRuntime] | Out-Null
$OPRESULT = [Windows.Networking.NetworkOperators.NetworkOperatorTetheringOperationResult]

function Get-Profiles {
    return @([Windows.Networking.Connectivity.NetworkInformation]::GetConnectionProfiles())
}

# Interfaces fisicas levantadas, excluyendo el propio adaptador virtual del
# hotspot y el TAP de OpenVPN.
function Get-ShareableAdapters {
    $list = @()
    foreach ($a in (Get-NetAdapter -Physical -ErrorAction SilentlyContinue)) {
        if ($a.Status -ne 'Up') { continue }
        if ($a.InterfaceDescription -like '*Wi-Fi Direct*') { continue }
        if ($a.InterfaceDescription -like '*TAP-Windows*') { continue }
        $list += $a
    }
    return $list
}

# Devuelve el perfil de Windows que corresponde a la interfaz indicada.
#
# Un mismo adaptador puede tener varios perfiles guardados (por ejemplo varias
# redes Wi-Fi), asi que primero preguntamos a Windows cual esta conectado de
# verdad y solo despues caemos al GUID del adaptador.
function Find-ProfileForAdapter($adapter, $profiles) {
    $activeName = $null
    try {
        $active = Get-NetConnectionProfile -InterfaceIndex $adapter.ifIndex -ErrorAction SilentlyContinue
        if ($active -ne $null) { $activeName = [string]$active.Name }
    } catch { $activeName = $null }
    if ($activeName -ne $null -and $activeName -ne '') {
        foreach ($p in $profiles) {
            if ($p.ProfileName -eq $activeName) { return $p }
        }
    }

    $guid = ([string]$adapter.InterfaceGuid).Trim('{', '}').ToLowerInvariant()
    $sameAdapter = @()
    foreach ($p in $profiles) {
        $na = $null
        try { $na = $p.NetworkAdapter } catch { $na = $null }
        if ($na -eq $null) { continue }
        $id = ''
        try { $id = ([string]$na.NetworkAdapterId).Trim('{', '}').ToLowerInvariant() } catch { continue }
        if ($id -eq $guid) { $sameAdapter += $p }
    }
    foreach ($p in $sameAdapter) {
        try {
            if ([string]$p.GetNetworkConnectivityLevel() -ne 'None') { return $p }
        } catch { }
    }
    if ($sameAdapter.Count -gt 0) { return $sameAdapter[0] }

    foreach ($p in $profiles) {
        if ($p.ProfileName -eq $adapter.Name) { return $p }
    }
    return $null
}

# Direcciones IPv4 del adaptador virtual del hotspot (normalmente 192.168.137.1).
function Get-HotspotAddresses {
    $out = @()
    foreach ($a in (Get-NetAdapter -ErrorAction SilentlyContinue)) {
        if ($a.InterfaceDescription -notlike '*Wi-Fi Direct*') { continue }
        if ($a.Status -ne 'Up') { continue }
        foreach ($ip in (Get-NetIPAddress -InterfaceIndex $a.ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue)) {
            $out += ($ip.IPAddress + '/' + $ip.PrefixLength)
        }
    }
    return $out
}

$profiles = Get-Profiles
$adapters = @(Get-ShareableAdapters)

# Preferimos el cable: es donde vive el proxy de la UCI. Si no hay cable, la
# Wi-Fi activa (por ejemplo la red UCI sin salida a internet).
$wired = @($adapters | Where-Object { $_.PhysicalMediaType -like '*802.3*' })
$adapter = $null
if ($wired.Count -gt 0) {
    $adapter = $wired[0]
} elseif ($adapters.Count -gt 0) {
    $adapter = $adapters[0]
}

# Perfil de Windows que corresponde a la interfaz que se va a compartir.
$profile = $null
if ($adapter -ne $null) {
    $profile = Find-ProfileForAdapter $adapter $profiles
}

# Cualquier perfil sirve para hablar con el servicio de anclaje cuando no hay
# nada conectado; hace falta para poder apagarlo igualmente.
$anyProfile = $profile
if ($anyProfile -eq $null) {
    try { $anyProfile = [Windows.Networking.Connectivity.NetworkInformation]::GetInternetConnectionProfile() } catch { $anyProfile = $null }
}
if ($anyProfile -eq $null -and $profiles.Count -gt 0) {
    $anyProfile = $profiles[0]
}
if ($anyProfile -eq $null) {
    Write-Output 'ERROR: Windows no expone ningun perfil de conexion. Conecta el cable o la Wi-Fi y vuelve a intentarlo.'
    exit 2
}

$managerProfile = $profile
if ($managerProfile -eq $null) { $managerProfile = $anyProfile }

$mgr = $null
try {
    $mgr = [Windows.Networking.NetworkOperators.NetworkOperatorTetheringManager]::CreateFromConnectionProfile($managerProfile)
} catch {
    Write-Output ('ERROR: no se pudo crear el gestor de anclaje: ' + $_.Exception.Message)
    exit 2
}
if ($mgr -eq $null) {
    Write-Output 'ERROR: Windows no devolvio un gestor de anclaje para ese perfil.'
    exit 2
}

$capability = 'Unknown'
try {
    $capability = [string][Windows.Networking.NetworkOperators.NetworkOperatorTetheringManager]::GetTetheringCapabilityFromConnectionProfile($managerProfile)
} catch {
    $capability = 'Unknown'
}

$state = 'Unknown'
try { $state = [string]$mgr.TetheringOperationalState } catch { $state = 'Unknown' }

$apSsid = ''
$apAuth = ''
try {
    $ap = $mgr.GetCurrentAccessPointConfiguration()
    $apSsid = [string]$ap.Ssid
    $apAuth = [string]$ap.AuthenticationKind
} catch { }

if ($Action -eq 'probe') {
    Write-Output ('STATE=' + $state)
    Write-Output ('CAPABILITY=' + $capability)
    exit 0
}

if ($Action -eq 'status') {
    Write-Output '=== PUNTO DE ACCESO MOVIL (WI-FI DIRECT / SOFTAP) ==='
    if ($adapter -ne $null) {
        Write-Output ('Interfaz a compartir : ' + $adapter.Name + ' (' + $adapter.InterfaceDescription + ')')
    } else {
        Write-Output 'Interfaz a compartir : ninguna activa'
    }
    $level = 'desconocido'
    try { $level = [string]$managerProfile.GetNetworkConnectivityLevel() } catch { }
    Write-Output ('Perfil de Windows   : ' + $managerProfile.ProfileName + ' | conectividad: ' + $level)
    Write-Output ('Capacidad de anclaje: ' + $capability)
    Write-Output ('Estado actual       : ' + $state)
    if ($apSsid -ne '') {
        Write-Output ('SSID configurado    : ' + $apSsid + ' | autenticacion: ' + $apAuth)
    }
    foreach ($addr in (Get-HotspotAddresses)) {
        Write-Output ('Direccion del AP    : ' + $addr)
    }
    Write-Output ''
    Write-Output 'Nota: "Hosted network supported: No" (netsh) es irrelevante en Windows 11.'
    Write-Output '      El punto de acceso usa Wi-Fi Direct y no exige que NCSI vea internet.'
    exit 0
}

if ($Action -eq 'start') {
    if ($state -eq 'On') {
        Write-Output ('El punto de acceso ya estaba activo (SSID: ' + $apSsid + '). No se modifico la configuracion.')
        exit 0
    }
    if ($adapter -eq $null) {
        Write-Output 'ERROR: no hay ninguna interfaz activa (Ethernet o Wi-Fi) que Windows pueda compartir. Conecta el cable o la Wi-Fi primero.'
        exit 2
    }
    if ($profile -eq $null) {
        Write-Output ('ERROR: no se encontro el perfil de Windows de la interfaz ' + $adapter.Name + '. Vuelve a conectar esa red e intentalo de nuevo.')
        exit 2
    }
    if ($capability -ne 'Enabled') {
        Write-Output ('ERROR: Windows no permite el anclaje con el perfil ' + $profile.ProfileName + ' (capacidad: ' + $capability + ').')
        exit 2
    }

    $result = $null
    try {
        $result = Await ($mgr.StartTetheringAsync()) $OPRESULT
    } catch {
        Write-Output ('ERROR: StartTetheringAsync fallo: ' + $_.Exception.Message)
        exit 3
    }
    if ([string]$result.Status -ne 'Success') {
        Write-Output ('ERROR: Windows rechazo el anclaje (' + [string]$result.Status + '): ' + [string]$result.AdditionalErrorMessage)
        exit 3
    }

    $on = $false
    for ($i = 0; $i -lt 20; $i++) {
        Start-Sleep -Milliseconds 400
        try {
            if ([string]$mgr.TetheringOperationalState -eq 'On') { $on = $true; break }
        } catch { }
    }
    if (-not $on) {
        try { Await ($mgr.StopTetheringAsync()) $OPRESULT | Out-Null } catch { }
        Write-Output 'ERROR: el punto de acceso no llego a estado On; se detuvo de nuevo (rollback).'
        exit 3
    }

    try { $apSsid = [string]$mgr.GetCurrentAccessPointConfiguration().Ssid } catch { }
    $level = 'desconocido'
    try { $level = [string]$profile.GetNetworkConnectivityLevel() } catch { }

    $msg = 'Hotspot activo. SSID: ' + $apSsid + ' | comparte: ' + $profile.ProfileName + ' (' + $level + ')'
    $addrs = @(Get-HotspotAddresses)
    if ($addrs.Count -gt 0) {
        $msg = $msg + ' | IP: ' + ($addrs -join ', ')
    }
    if ($level -eq 'None' -or $level -eq 'LocalAccess') {
        $msg = $msg + ' | El origen no tiene internet: los clientes tendran red local y podran usar el proxy de la UCI si lo configuran.'
    }
    Write-Output $msg
    exit 0
}

if ($Action -eq 'stop') {
    try {
        Await ($mgr.StopTetheringAsync()) $OPRESULT | Out-Null
    } catch {
        Write-Output ('AVISO: StopTetheringAsync devolvio un error: ' + $_.Exception.Message)
    }

    $off = $false
    for ($i = 0; $i -lt 10; $i++) {
        try {
            if ([string]$mgr.TetheringOperationalState -eq 'Off') { $off = $true; break }
        } catch { }
        Start-Sleep -Milliseconds 300
    }

    if (-not $off) {
        $now = 'Unknown'
        try { $now = [string]$mgr.TetheringOperationalState } catch { }
        Write-Output ('ERROR: el punto de acceso sigue en estado ' + $now + '. Apagalo desde Configuracion > Punto de acceso movil.')
        exit 3
    }

    Write-Output 'Hotspot detenido. La configuracion de red de Windows no se modifico.'
    exit 0
}

Write-Output ('ERROR: accion desconocida: ' + $Action)
exit 2
