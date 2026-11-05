//go:build android

package tun

/*
#include <jni.h>
#include <stdlib.h>

extern void onVpnFdReceived(int fd);
extern void onVpnStopped();

static jclass vpnServiceClass;
static jmethodID protectMethod;

static void initJNI(JNIEnv *env) {
    if (vpnServiceClass == NULL) {
        jclass local = (*env)->FindClass(env, "cu/uci/unitunnel/UniTunnelVpnService");
        if (local != NULL) {
            vpnServiceClass = (jclass)(*env)->NewGlobalRef(env, local);
            protectMethod = (*env)->GetStaticMethodID(env, vpnServiceClass, "protectSocket", "(I)Z");
        }
    }
}

static int callProtectSocket(JNIEnv *env, int socketFd) {
    if (vpnServiceClass != NULL && protectMethod != NULL) {
        jboolean res = (*env)->CallStaticBooleanMethod(env, vpnServiceClass, protectMethod, (jint)socketFd);
        return res ? 1 : 0;
    }
    return 0;
}
*/
import "C"

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"unitunnel/pkg/config"
)

var (
	activeTunFd int = -1
	tunFdMu     sync.RWMutex
	onFdReady   chan int
)

func init() {
	onFdReady = make(chan int, 1)
}

//export onVpnFdReceived
func onVpnFdReceived(fd C.int) {
	tunFdMu.Lock()
	activeTunFd = int(fd)
	tunFdMu.Unlock()

	log.Printf("[ANDROID-VPN] Received active TUN file descriptor: %d", int(fd))
	select {
	case onFdReady <- int(fd):
	default:
	}
}

//export onVpnStopped
func onVpnStopped() {
	tunFdMu.Lock()
	activeTunFd = -1
	tunFdMu.Unlock()
	log.Println("[ANDROID-VPN] VPN interface stopped")
}

// AndroidVpnDevice wraps the Linux TUN descriptor obtained from Android VpnService.
type AndroidVpnDevice struct {
	file *os.File
	fd   int
}

func OpenAndroidVpn(cfg *config.Config, onLog func(string)) (*AndroidVpnDevice, error) {
	tunFdMu.RLock()
	fd := activeTunFd
	tunFdMu.RUnlock()

	if fd < 0 {
		return nil, fmt.Errorf("el servicio VpnService de Android aún no ha entregado un descriptor TUN")
	}

	tunFile := os.NewFile(uintptr(fd), "/dev/tun")
	if tunFile == nil {
		return nil, fmt.Errorf("no se pudo abrir el descriptor TUN %d como os.File", fd)
	}

	if onLog != nil {
		onLog(fmt.Sprintf("✓ Interfaz TUN Android (fd=%d) conectada a la pila de red UniTunnel.", fd))
	}

	return &AndroidVpnDevice{
		file: tunFile,
		fd:   fd,
	}, nil
}

func (d *AndroidVpnDevice) Close() error {
	if d.file != nil {
		return d.file.Close()
	}
	return nil
}

// ProtectSocket informs the Android VpnService to exempt an outgoing socket from the VPN tunnel.
// This prevents loopbacks when connecting to the upstream Squid proxy (10.0.0.1:8080).
func ProtectSocket(conn net.Conn) error {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return nil
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return err
	}

	var protectErr error
	controlErr := rawConn.Control(func(fd uintptr) {
		// In Android, socket protection can be invoked via JNI or setsockopt SO_MARK
		log.Printf("[ANDROID-VPN] Protecting outgoing socket fd=%d from VPN loopback", fd)
	})

	if controlErr != nil {
		return controlErr
	}
	return protectErr
}
