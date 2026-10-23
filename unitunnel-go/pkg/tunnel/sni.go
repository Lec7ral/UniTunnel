package tunnel

import (
	"encoding/binary"
	"net"
	"strings"
	"sync"
	"time"
)

// DNSCache stores bidirectional mappings between resolved IPs and their FQDN domain names
type DNSCache struct {
	mu      sync.RWMutex
	ipToDom map[string]dnsEntry
}

type dnsEntry struct {
	domain    string
	expiresAt time.Time
}

var globalDNSCache = &DNSCache{
	ipToDom: make(map[string]dnsEntry),
}

// RecordDNSMapping saves an IP -> Domain association (default TTL: 30 minutes)
func RecordDNSMapping(ip, domain string) {
	if ip == "" || domain == "" {
		return
	}
	globalDNSCache.mu.Lock()
	defer globalDNSCache.mu.Unlock()

	globalDNSCache.ipToDom[ip] = dnsEntry{
		domain:    strings.TrimSuffix(domain, "."),
		expiresAt: time.Now().Add(30 * time.Minute),
	}
}

// LookupDomainByIP retrieves the associated domain for an IP address if known
func LookupDomainByIP(ip string) string {
	globalDNSCache.mu.RLock()
	defer globalDNSCache.mu.RUnlock()

	entry, found := globalDNSCache.ipToDom[ip]
	if !found {
		return ""
	}
	if time.Now().After(entry.expiresAt) {
		return ""
	}
	return entry.domain
}

// ExtractSNI parses the Server Name Indication (SNI) from a TLS ClientHello packet.
// This allows converting direct IP connections (e.g. 142.251.211.238:443) into FQDN domain
// requests (e.g. google.com:443) that bypass Squid's "numeric_IPs" block.
func ExtractSNI(data []byte) string {
	// Minimum length for a valid TLS ClientHello with extensions is ~44 bytes
	if len(data) < 44 {
		return ""
	}

	// Byte 0: ContentType must be Handshake (0x16)
	if data[0] != 0x16 {
		return ""
	}

	// Byte 1-2: Record version (0x03, 0x01 through 0x03, 0x04)
	if data[1] != 0x03 {
		return ""
	}

	// Byte 5: HandshakeType must be ClientHello (0x01)
	if data[5] != 0x01 {
		return ""
	}

	// Skip Record Header (5 bytes) + Handshake Type (1 byte) + Length (3 bytes) + Version (2 bytes) + Random (32 bytes)
	offset := 5 + 1 + 3 + 2 + 32
	if len(data) <= offset {
		return ""
	}

	// Session ID length
	sessionIDLen := int(data[offset])
	offset += 1 + sessionIDLen
	if len(data) <= offset+2 {
		return ""
	}

	// Cipher Suites length
	cipherSuitesLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2 + cipherSuitesLen
	if len(data) <= offset+1 {
		return ""
	}

	// Compression Methods length
	compressionLen := int(data[offset])
	offset += 1 + compressionLen
	if len(data) <= offset+2 {
		return ""
	}

	// Extensions length
	extensionsLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2
	end := offset + extensionsLen
	if end > len(data) {
		end = len(data)
	}

	// Parse TLS Extensions
	for offset+4 <= end {
		extType := binary.BigEndian.Uint16(data[offset : offset+2])
		extLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		offset += 4

		if offset+extLen > end {
			break
		}

		// Extension 0x0000 = server_name (SNI)
		if extType == 0 {
			sniData := data[offset : offset+extLen]
			if len(sniData) < 5 {
				break
			}
			// ServerNameList length (2 bytes)
			// ServerName Type (1 byte, 0x00 = host_name)
			if sniData[2] == 0 {
				nameLen := int(binary.BigEndian.Uint16(sniData[3:5]))
				if len(sniData) >= 5+nameLen {
					return string(sniData[5 : 5+nameLen])
				}
			}
		}

		offset += extLen
	}

	return ""
}

// ParseAndRecordDNSResponse extracts IPv4 A records from standard RFC 1035 responses
// and registers them into the reverse DNS cache.
func ParseAndRecordDNSResponse(resp []byte) {
	if len(resp) < 12 {
		return
	}
	qdCount := binary.BigEndian.Uint16(resp[4:6])
	anCount := binary.BigEndian.Uint16(resp[6:8])
	if anCount == 0 {
		return
	}

	offset := 12
	var questionDomain string
	for i := 0; i < int(qdCount); i++ {
		dom, nextOffset := readDNSName(resp, offset)
		if dom != "" && questionDomain == "" {
			questionDomain = dom
		}
		offset = nextOffset + 4 // skip QTYPE (2) + QCLASS (2)
		if offset >= len(resp) {
			return
		}
	}

	if questionDomain == "" {
		return
	}

	for i := 0; i < int(anCount); i++ {
		if offset >= len(resp) {
			break
		}
		_, nextOffset := readDNSName(resp, offset)
		offset = nextOffset
		if offset+10 > len(resp) {
			break
		}
		rtype := binary.BigEndian.Uint16(resp[offset : offset+2])
		rdLen := int(binary.BigEndian.Uint16(resp[offset+8 : offset+10]))
		offset += 10

		if offset+rdLen > len(resp) {
			break
		}

		if rtype == 1 && rdLen == 4 { // Type A (IPv4)
			ip := net.IP(resp[offset : offset+4]).String()
			RecordDNSMapping(ip, questionDomain)
		}
		offset += rdLen
	}
}

func readDNSName(data []byte, offset int) (string, int) {
	var parts []string
	curr := offset
	jumped := false
	jumpOffset := 0

	for curr < len(data) {
		length := int(data[curr])
		if length == 0 {
			curr++
			break
		}
		if length >= 192 { // Pointer 0xc0
			if curr+1 >= len(data) {
				break
			}
			if !jumped {
				jumpOffset = curr + 2
				jumped = true
			}
			curr = int(binary.BigEndian.Uint16(data[curr:curr+2]) & 0x3fff)
			continue
		}
		curr++
		if curr+length > len(data) {
			break
		}
		parts = append(parts, string(data[curr:curr+length]))
		curr += length
	}

	if jumped {
		return strings.Join(parts, "."), jumpOffset
	}
	return strings.Join(parts, "."), curr
}
