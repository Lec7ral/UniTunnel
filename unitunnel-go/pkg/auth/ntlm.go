package auth

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"time"
	"unicode/utf16"
)

// NTLM Message Types
const (
	NtLmNegotiate    = 1
	NtLmChallenge    = 2
	NtLmAuthenticate = 3
)

// NTLM Negotiate Flags (MS-NLMP)
const (
	NegotiateUnicode               = 0x00000001
	NegotiateOEM                   = 0x00000002
	RequestTarget                  = 0x00000004
	NegotiateNTLM                  = 0x00000200
	NegotiateDomainSupplied        = 0x00001000
	NegotiateWorkstationSupplied   = 0x00002000
	NegotiateAlwaysSign            = 0x00008000
	NegotiateExtendedSessionSec    = 0x00080000
	NegotiateTargetInfo            = 0x00800000
	NegotiateVersion               = 0x02000000
	Negotiate128                   = 0x20000000
	NegotiateKeyExchange           = 0x40000000
	Negotiate56                    = 0x80000000
)

var ntlmSignature = []byte("NTLMSSP\x00")

// SplitDomainUser extracts domain and username from composite formats like DOMAIN\user or user@domain
func SplitDomainUser(domain, username string) (string, string) {
	username = strings.TrimSpace(username)
	domain = strings.TrimSpace(domain)

	if strings.Contains(username, `\`) {
		parts := strings.SplitN(username, `\`, 2)
		if domain == "" {
			domain = parts[0]
		}
		username = parts[1]
	} else if strings.Contains(username, "/") {
		parts := strings.SplitN(username, "/", 2)
		if domain == "" {
			domain = parts[0]
		}
		username = parts[1]
	} else if strings.Contains(username, "@") {
		parts := strings.SplitN(username, "@", 2)
		username = parts[0]
		if domain == "" {
			domain = parts[1]
		}
	}
	return domain, username
}

// CreateNegotiateMessage (Type 1)
func CreateNegotiateMessage(domain, workstation string) string {
	cleanDomain, _ := SplitDomainUser(domain, "")
	if workstation == "" {
		workstation = "UNITUNNEL-PC"
	}

	flags := uint32(NegotiateUnicode | NegotiateOEM | RequestTarget | NegotiateNTLM |
		NegotiateAlwaysSign | NegotiateExtendedSessionSec | Negotiate128 | Negotiate56)

	domainBytes := []byte(strings.ToUpper(cleanDomain))
	workstationBytes := []byte(strings.ToUpper(workstation))

	if len(domainBytes) > 0 {
		flags |= NegotiateDomainSupplied
	}
	if len(workstationBytes) > 0 {
		flags |= NegotiateWorkstationSupplied
	}

	buf := new(bytes.Buffer)
	buf.Write(ntlmSignature)
	_ = binary.Write(buf, binary.LittleEndian, uint32(NtLmNegotiate))
	_ = binary.Write(buf, binary.LittleEndian, flags)

	// Domain security buffer
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(domainBytes)))
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(domainBytes)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(32+len(workstationBytes)))

	// Workstation security buffer
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(workstationBytes)))
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(workstationBytes)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(32))

	buf.Write(workstationBytes)
	buf.Write(domainBytes)

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// ChallengeDetails contains parsed info from NTLM Type-2 Challenge
type ChallengeDetails struct {
	ServerChallenge []byte
	TargetInfo      []byte
	TargetName      string
	Flags           uint32
}

// ParseChallengeMessage (Type 2) parses challenge, target info and domain
func ParseChallengeMessage(b64Message string) (*ChallengeDetails, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Message))
	if err != nil {
		return nil, err
	}

	if len(data) < 32 || !bytes.Equal(data[:8], ntlmSignature) {
		return nil, err
	}

	res := &ChallengeDetails{
		ServerChallenge: make([]byte, 8),
		Flags:           binary.LittleEndian.Uint32(data[20:24]),
	}
	copy(res.ServerChallenge, data[24:32])

	// Extract Target Name (Domain) if present
	if len(data) >= 20 {
		targetNameLen := binary.LittleEndian.Uint16(data[12:14])
		targetNameOffset := binary.LittleEndian.Uint32(data[16:20])
		if targetNameLen > 0 && int(targetNameOffset+uint32(targetNameLen)) <= len(data) {
			rawName := data[targetNameOffset : targetNameOffset+uint32(targetNameLen)]
			res.TargetName = fromUnicode(rawName)
		}
	}

	// Extract TargetInfo block (offset 40..48) for NTLMv2 compatibility
	if len(data) >= 48 {
		targetInfoLen := binary.LittleEndian.Uint16(data[40:42])
		targetInfoOffset := binary.LittleEndian.Uint32(data[44:48])
		if targetInfoLen > 0 && int(targetInfoOffset+uint32(targetInfoLen)) <= len(data) {
			res.TargetInfo = make([]byte, targetInfoLen)
			copy(res.TargetInfo, data[targetInfoOffset:targetInfoOffset+uint32(targetInfoLen)])
		}
	}

	return res, nil
}

// CreateAuthenticateMessage (Type 3) generates authentic NTLMv2 (Active Directory standard)
func CreateAuthenticateMessage(domain, username, password, workstation string, challenge []byte, targetInfo []byte) string {
	cleanDomain, cleanUser := SplitDomainUser(domain, username)
	if workstation == "" {
		workstation = "UNITUNNEL-PC"
	}

	// 1. Compute NT Hash = MD4(UTF-16LE(password))
	ntHash := md4Sum(toUnicode(password))

	// 2. Compute NTLMv2 Hash = HMAC-MD5(key = ntHash, data = UTF-16LE(UPPER(cleanUser) + cleanDomain))
	userUnicodeUpper := toUnicode(strings.ToUpper(cleanUser))
	domainUnicodeUpper := toUnicode(strings.ToUpper(cleanDomain))

	macV2 := hmac.New(md5.New, ntHash)
	macV2.Write(userUnicodeUpper)
	macV2.Write(domainUnicodeUpper)
	ntlmV2Hash := macV2.Sum(nil)

	// 3. Generate 8-byte random client nonce
	clientNonce := make([]byte, 8)
	if _, err := rand.Read(clientNonce); err != nil {
		binary.LittleEndian.PutUint64(clientNonce, uint64(time.Now().UnixNano()))
	}

	// 4. Construct NTLMv2 Client Blob
	// Blob format: Signature (0x01010000) + Reserved + Timestamp + ClientNonce + Reserved + TargetInfo + EndReserved
	blob := new(bytes.Buffer)
	blob.Write([]byte{0x01, 0x01, 0x00, 0x00}) // Signature
	blob.Write([]byte{0x00, 0x00, 0x00, 0x00}) // Reserved

	// Windows FileTime: 100-nanosecond intervals since January 1, 1601 UTC
	fileTime := uint64((time.Now().UnixNano() / 100) + 116444736000000000)
	_ = binary.Write(blob, binary.LittleEndian, fileTime)

	blob.Write(clientNonce)
	blob.Write([]byte{0x00, 0x00, 0x00, 0x00}) // Reserved

	if len(targetInfo) > 0 {
		blob.Write(targetInfo)
	}
	blob.Write([]byte{0x00, 0x00, 0x00, 0x00}) // End Reserved

	blobBytes := blob.Bytes()

	// 5. Compute NTLMv2 Response = HMAC-MD5(ntlmV2Hash, challenge + blob) + blob
	macNTLM := hmac.New(md5.New, ntlmV2Hash)
	macNTLM.Write(challenge)
	macNTLM.Write(blobBytes)
	ntlmProof := macNTLM.Sum(nil)

	ntlmResponse := append(ntlmProof, blobBytes...)

	// 6. Compute LMv2 Response = HMAC-MD5(ntlmV2Hash, challenge + clientNonce) + clientNonce
	macLM := hmac.New(md5.New, ntlmV2Hash)
	macLM.Write(challenge)
	macLM.Write(clientNonce)
	lmProof := macLM.Sum(nil)
	lmResponse := append(lmProof, clientNonce...)

	userUnicode := toUnicode(cleanUser)
	domainUnicode := toUnicode(strings.ToUpper(cleanDomain))
	workstationUnicode := toUnicode(strings.ToUpper(workstation))

	flags := uint32(NegotiateUnicode | RequestTarget | NegotiateNTLM |
		NegotiateAlwaysSign | NegotiateExtendedSessionSec | Negotiate128 | Negotiate56)

	payloadOffset := 64
	domainOffset := payloadOffset
	payloadOffset += len(domainUnicode)
	userOffset := payloadOffset
	payloadOffset += len(userUnicode)
	workstationOffset := payloadOffset
	payloadOffset += len(workstationUnicode)
	lmOffset := payloadOffset
	payloadOffset += len(lmResponse)
	ntlmOffset := payloadOffset

	buf := new(bytes.Buffer)
	buf.Write(ntlmSignature)
	_ = binary.Write(buf, binary.LittleEndian, uint32(NtLmAuthenticate))

	// LMv2 response buffer
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(lmResponse)))
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(lmResponse)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(lmOffset))

	// NTLMv2 response buffer
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(ntlmResponse)))
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(ntlmResponse)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(ntlmOffset))

	// Domain buffer
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(domainUnicode)))
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(domainUnicode)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(domainOffset))

	// User buffer
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(userUnicode)))
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(userUnicode)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(userOffset))

	// Workstation buffer
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(workstationUnicode)))
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(workstationUnicode)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(workstationOffset))

	// Session key buffer (empty)
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(buf, binary.LittleEndian, uint32(payloadOffset+len(ntlmResponse)))

	// Flags
	_ = binary.Write(buf, binary.LittleEndian, flags)

	// Append payloads
	buf.Write(domainUnicode)
	buf.Write(userUnicode)
	buf.Write(workstationUnicode)
	buf.Write(lmResponse)
	buf.Write(ntlmResponse)

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func toUnicode(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	buf := new(bytes.Buffer)
	for _, v := range u16 {
		_ = binary.Write(buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

func fromUnicode(b []byte) string {
	if len(b)%2 != 0 {
		return string(b)
	}
	u16 := make([]uint16, len(b)/2)
	for i := 0; i < len(u16); i++ {
		u16[i] = binary.LittleEndian.Uint16(b[i*2 : i*2+2])
	}
	return string(utf16.Decode(u16))
}

// md4Sum implements RFC 1320 MD4 message-digest algorithm (for NT-Hash)
func md4Sum(msg []byte) []byte {
	origLenBits := uint64(len(msg) * 8)

	// Padding: append 0x80, then zeroes until len % 64 == 56, then 64-bit length
	padded := append([]byte(nil), msg...)
	padded = append(padded, 0x80)
	for len(padded)%64 != 56 {
		padded = append(padded, 0x00)
	}
	lenBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(lenBuf, origLenBits)
	padded = append(padded, lenBuf...)

	// Initial constants
	a := uint32(0x67452301)
	b := uint32(0xefcdab89)
	c := uint32(0x98badcfe)
	d := uint32(0x10325476)

	f := func(x, y, z uint32) uint32 { return (x & y) | (^x & z) }
	g := func(x, y, z uint32) uint32 { return (x & y) | (x & z) | (y & z) }
	h := func(x, y, z uint32) uint32 { return x ^ y ^ z }
	rot := func(x uint32, s uint) uint32 { return (x << s) | (x >> (32 - s)) }

	s1 := []uint{3, 7, 11, 19}
	s2 := []uint{3, 5, 9, 13}
	s3 := []uint{3, 9, 11, 15}
	r3Order := []int{0, 8, 4, 12, 2, 10, 6, 14, 1, 9, 5, 13, 3, 11, 7, 15}

	for chunk := 0; chunk < len(padded); chunk += 64 {
		var x [16]uint32
		for j := 0; j < 16; j++ {
			x[j] = binary.LittleEndian.Uint32(padded[chunk+j*4 : chunk+j*4+4])
		}

		aa, bb, cc, dd := a, b, c, d

		// Round 1
		for j := 0; j < 16; j++ {
			a = rot(a+f(b, c, d)+x[j], s1[j%4])
			a, b, c, d = d, a, b, c
		}

		// Round 2
		for j := 0; j < 16; j++ {
			k := (j%4)*4 + (j / 4)
			a = rot(a+g(b, c, d)+x[k]+0x5a827999, s2[j%4])
			a, b, c, d = d, a, b, c
		}

		// Round 3
		for j := 0; j < 16; j++ {
			k := r3Order[j]
			a = rot(a+h(b, c, d)+x[k]+0x6ed9eba1, s3[j%4])
			a, b, c, d = d, a, b, c
		}

		a += aa
		b += bb
		c += cc
		d += dd
	}

	out := make([]byte, 16)
	binary.LittleEndian.PutUint32(out[0:4], a)
	binary.LittleEndian.PutUint32(out[4:8], b)
	binary.LittleEndian.PutUint32(out[8:12], c)
	binary.LittleEndian.PutUint32(out[12:16], d)
	return out
}
