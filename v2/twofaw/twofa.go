// Package twofaw provides TOTP-based two-factor authentication utilities.
package twofaw

import (
	"context"
)

// TwoFA provides TOTP secret provisioning, code validation, and recovery codes.
type TwoFA interface {
	// GenerateKey creates a new TOTP key for the given account and issuer.
	GenerateKey(ctx context.Context, accountName, issuer string) (*Key, error)

	// GenerateQRCode returns a PNG QR code encoding the key provisioning URI.
	GenerateQRCode(ctx context.Context, key *Key, size int) ([]byte, error)

	// ValidateCode verifies a TOTP code against a base32-encoded secret.
	ValidateCode(ctx context.Context, secret, code string, skew uint) bool

	// GenerateRecoveryCodes creates one-time recovery codes.
	// It returns the plain codes and their SHA-256 hashes.
	GenerateRecoveryCodes(ctx context.Context, count int) (plain []string, hashed []string, err error)

	// ValidateRecoveryCode checks a recovery code against a list of hashed codes.
	// On success it returns true and the index of the matched hash.
	ValidateRecoveryCode(ctx context.Context, code string, hashedCodes []string) (bool, int)
}

// Key holds a generated TOTP secret and provisioning URI.
type Key struct {
	Secret string // base32-encoded secret
	URL    string // otpauth:// provisioning URI
}

// Config controls TOTP generation parameters.
type Config struct {
	// Issuer is the service name shown in authenticator apps.
	// Defaults to "go-utility" if empty.
	Issuer string

	// Digits is the number of digits in generated codes.
	// Defaults to 6 if zero.
	Digits int

	// Period is the code validity window in seconds.
	// Defaults to 30 if zero.
	Period uint

	// Algorithm is the HMAC algorithm to use.
	// Defaults to SHA1 if empty.
	Algorithm string
}
