package traffic

import (
	"io"
	"net"
	"sync/atomic"
)

var (
	totalBytesRead    uint64
	totalBytesWritten uint64
	lastPingMS        int64
)

func AddRead(n int) {
	atomic.AddUint64(&totalBytesRead, uint64(n))
}

func AddWrite(n int) {
	atomic.AddUint64(&totalBytesWritten, uint64(n))
}

func SetPing(ms int64) {
	atomic.StoreInt64(&lastPingMS, ms)
}

func GetPing() int64 {
	return atomic.LoadInt64(&lastPingMS)
}

func GetTotals() (readMB float64, writeMB float64, totalMB float64) {
	r := atomic.LoadUint64(&totalBytesRead)
	w := atomic.LoadUint64(&totalBytesWritten)
	return float64(r) / (1024 * 1024), float64(w) / (1024 * 1024), float64(r+w) / (1024 * 1024)
}

// CountingPipe forwards bidirectional traffic while measuring bytes.
//
// Semantics match the UI: bytes flowing from dst to src are a *download* and
// are reported as read_mb; bytes flowing from src to dst are an *upload* and
// are reported as write_mb. Callers pass (localConn, remoteConn).
func CountingPipe(src, dst net.Conn) {
	errChan := make(chan error, 2)

	// Upload direction: local client -> remote server.
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				AddWrite(n)
				if _, wErr := dst.Write(buf[:n]); wErr != nil {
					errChan <- wErr
					return
				}
			}
			if err != nil {
				if err == io.EOF {
					halfCloseWrite(dst)
					errChan <- nil
				} else {
					errChan <- err
				}
				return
			}
		}
	}()

	// Download direction: remote server -> local client.
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := dst.Read(buf)
			if n > 0 {
				AddRead(n)
				if _, wErr := src.Write(buf[:n]); wErr != nil {
					errChan <- wErr
					return
				}
			}
			if err != nil {
				if err == io.EOF {
					halfCloseWrite(src)
					errChan <- nil
				} else {
					errChan <- err
				}
				return
			}
		}
	}()

	if err := <-errChan; err != nil {
		// Hard error: the other direction cannot progress either.
		_ = src.Close()
		_ = dst.Close()
		return
	}
	// Clean half-close: let the remaining direction drain before closing.
	<-errChan
	_ = src.Close()
	_ = dst.Close()
}

// halfCloseWrite propagates EOF without tearing down the reverse direction.
func halfCloseWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
}
