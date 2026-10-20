package traffic

import (
	"io"
	"net"
	"sync/atomic"
	"time"
)

var (
	totalBytesRead    uint64
	totalBytesWritten uint64
	currentSpeedMBs   uint64 // bits for float64
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

// CountingPipe forwards bidirectional traffic while measuring bytes
func CountingPipe(src, dst net.Conn) {
	defer src.Close()
	defer dst.Close()

	errChan := make(chan error, 2)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := src.Read(buf)
			if n > 0 {
				AddRead(n)
				_, wErr := dst.Write(buf[:n])
				if wErr != nil {
					errChan <- wErr
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					errChan <- err
				} else {
					errChan <- nil
				}
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := dst.Read(buf)
			if n > 0 {
				AddWrite(n)
				_, wErr := src.Write(buf[:n])
				if wErr != nil {
					errChan <- wErr
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					errChan <- err
				} else {
					errChan <- nil
				}
				return
			}
		}
	}()

	<-errChan
}

// StartSpeedMonitor calculates real-time MB/s every interval
func StartSpeedMonitor(interval time.Duration, onTick func(speedMBs float64, pingMS int64)) chan struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var lastTotal uint64
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				r := atomic.LoadUint64(&totalBytesRead)
				w := atomic.LoadUint64(&totalBytesWritten)
				curr := r + w

				delta := curr - lastTotal
				lastTotal = curr

				speed := (float64(delta) / (1024 * 1024)) / interval.Seconds()
				ping := GetPing()

				if onTick != nil {
					onTick(speed, ping)
				}
			}
		}
	}()
	return stop
}
