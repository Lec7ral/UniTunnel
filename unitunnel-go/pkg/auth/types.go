package auth

// AuthType identifies the proxy authentication scheme.
type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeBasic  AuthType = "basic"
	AuthTypeNTLM   AuthType = "ntlm"
	AuthTypeDigest AuthType = "digest"
)
