package twofaw

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/AndreeJait/go-utility/v2/logw"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
)

var _ TwoFA = (*totpManager)(nil)

type totpManager struct {
	cfg *Config
}

// New creates a TwoFA manager from the provided configuration.
func New(cfg *Config) TwoFA {
	if cfg == nil {
		cfg = &Config{}
	}
	return &totpManager{cfg: cfg}
}

func (m *totpManager) GenerateKey(ctx context.Context, accountName, issuer string) (*Key, error) {
	if accountName == "" {
		return nil, fmt.Errorf("twofaw: account name is required")
	}
	if issuer == "" {
		issuer = m.cfg.issuer()
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Period:      m.cfg.period(),
		Digits:      otp.Digits(m.cfg.digits()),
		Algorithm:   algorithm(m.cfg.Algorithm),
	})
	if err != nil {
		return nil, fmt.Errorf("twofaw: generate key: %w", err)
	}

	return &Key{Secret: key.Secret(), URL: key.URL()}, nil
}

func (m *totpManager) GenerateQRCode(ctx context.Context, key *Key, size int) ([]byte, error) {
	if key == nil || key.URL == "" {
		return nil, fmt.Errorf("twofaw: key URL is required")
	}
	if size <= 0 {
		size = 256
	}
	png, err := qrcode.Encode(key.URL, qrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("twofaw: generate qr code: %w", err)
	}
	return png, nil
}

func (m *totpManager) ValidateCode(ctx context.Context, secret, code string, skew uint) bool {
	if secret == "" || code == "" {
		return false
	}
	if skew == 0 {
		skew = 1
	}
	valid, _ := totp.ValidateCustom(code, secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    m.cfg.period(),
		Skew:      uint(skew),
		Digits:    otp.Digits(m.cfg.digits()),
		Algorithm: algorithm(m.cfg.Algorithm),
	})
	return valid
}

func (m *totpManager) GenerateRecoveryCodes(ctx context.Context, count int) (plain []string, hashed []string, err error) {
	if count <= 0 {
		return nil, nil, fmt.Errorf("twofaw: recovery code count must be positive")
	}

	plain = make([]string, 0, count)
	hashed = make([]string, 0, count)
	for i := 0; i < count; i++ {
		code := randomRecoveryCode()
		plain = append(plain, code)
		hashed = append(hashed, hashRecoveryCode(code))
	}
	return plain, hashed, nil
}

func (m *totpManager) ValidateRecoveryCode(ctx context.Context, code string, hashedCodes []string) (bool, int) {
	if code == "" || len(hashedCodes) == 0 {
		return false, -1
	}
	hash := hashRecoveryCode(code)
	for i, h := range hashedCodes {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(h)) == 1 {
			return true, i
		}
	}
	return false, -1
}

func algorithm(name string) otp.Algorithm {
	switch strings.ToUpper(name) {
	case "SHA256":
		return otp.AlgorithmSHA256
	case "SHA512":
		return otp.AlgorithmSHA512
	case "SHA1", "":
		return otp.AlgorithmSHA1
	default:
		logw.CtxInfof(context.Background(), "twofaw: unsupported algorithm %q, defaulting to SHA1", name)
		return otp.AlgorithmSHA1
	}
}

func hashRecoveryCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

const recoveryCodeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func randomRecoveryCode() string {
	const groups = 4
	const groupSize = 4
	groupsList := make([]string, groups)
	for i := range groupsList {
		b := make([]byte, groupSize)
		for j := range b {
			b[j] = recoveryCodeAlphabet[rand.Intn(len(recoveryCodeAlphabet))]
		}
		groupsList[i] = string(b)
	}
	return strings.Join(groupsList, "-")
}
