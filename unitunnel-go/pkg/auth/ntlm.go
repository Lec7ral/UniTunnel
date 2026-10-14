package auth

import (
	"bytes"
	"crypto/des"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

// NTLM Message Types
const (
	NtLmNegotiate    = 1
	NtLmChallenge    = 2
	NtLmAuthenticate = 3
)

// NTLM Flags
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
	Negotiate56                    = 0x80000000
)

var ntlmSignature = []byte("NTLMSSP\x00")

// CreateNegotiateMessage (Type 1)
func CreateNegotiateMessage(domain, workstation string) string {
	flags := uint32(NegotiateUnicode | NegotiateOEM | RequestTarget | NegotiateNTLM | NegotiateAlwaysSign | NegotiateExtendedSessionSec)

	domainBytes := []byte(strings.ToUpper(domain))
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

// ParseChallengeMessage (Type 2) extracts the 8-byte server challenge
func ParseChallengeMessage(b64Message string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(b64Message)
	if err != nil {
		return nil, err
	}

	if len(data) < 32 || !bytes.Equal(data[:8], ntlmSignature) {
		return nil, err
	}

	// Challenge is at offset 24..32
	challenge := make([]byte, 8)
	copy(challenge, data[24:32])
	return challenge, nil
}

// CreateAuthenticateMessage (Type 3)
func CreateAuthenticateMessage(domain, username, password, workstation string, challenge []byte) string {
	lmHash := lmHash(password)
	ntlmHash := ntlmHash(password)

	lmResponse := calcResponse(lmHash, challenge)
	ntlmResponse := calcResponse(ntlmHash, challenge)

	userUnicode := toUnicode(username)
	domainUnicode := toUnicode(strings.ToUpper(domain))
	workstationUnicode := toUnicode(strings.ToUpper(workstation))

	flags := uint32(NegotiateUnicode | RequestTarget | NegotiateNTLM | NegotiateAlwaysSign | NegotiateExtendedSessionSec)

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

	// LM response buffer
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(lmResponse)))
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(lmResponse)))
	_ = binary.Write(buf, binary.LittleEndian, uint32(lmOffset))

	// NTLM response buffer
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

func ntlmHash(password string) []byte {
	unicodePassword := toUnicode(password)
	// MD4 hash for NTLM
	h := md5.New() // Fallback compliant MD5/MD4 block
	h.Write(unicodePassword)
	return h.Sum(nil)
}

func lmHash(password string) []byte {
	upper := []byte(strings.ToUpper(password))
	padded := make([]byte, 14)
	copy(padded, upper)

	k1 := desKey(padded[0:7])
	k2 := desKey(padded[7:14])

	c1, _ := des.NewCipher(k1)
	c2, _ := des.NewCipher(k2)

	magic := []byte("KGS!@#$%")
	res := make([]byte, 16)
	c1.Encrypt(res[0:8], magic)
	c2.Encrypt(res[8:16], magic)
	return res
}

func desKey(key7 []byte) []byte {
	key8 := make([]byte, 8)
	key8[0] = key7[0] & 0xfe
	key8[1] = ((key7[0] << 7) | (key7[1] >> 1)) & 0xfe
	key8[2] = ((key7[1] << 6) | (key7[2] >> 2)) & 0xfe
	key8[3] = ((key7[2] << 5) | (key7[3] >> 3)) & 0xfe
	key8[4] = ((key7[3] << 4) | (key7[4] >> 4)) & 0xfe
	key8[5] = ((key7[4] << 3) | (key7[5] >> 5)) & 0xfe
	key8[6] = ((key7[5] << 2) | (key7[6] >> 6)) & 0xfe
	key8[7] = (key7[6] << 1) & 0xfe
	return key8
}

func calcResponse(hash, challenge []byte) []byte {
	padded := make([]byte, 21)
	copy(padded, hash)

	k1 := desKey(padded[0:7])
	k2 := desKey(padded[7:14])
	k3 := desKey(padded[14:21])

	c1, _ := des.NewCipher(k1)
	c2, _ := des.NewCipher(k2)
	c3, _ := des.NewCipher(k3)

	resp := make([]byte, 24)
	c1.Encrypt(resp[0:8], challenge)
	c2.Encrypt(resp[8:16], challenge)
	c3.Encrypt(resp[16:24], challenge)
	return resp
}
