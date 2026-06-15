package twofaw

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestNew_DefaultConfig(t *testing.T) {
	m := New(nil)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestGenerateKey_MissingAccountName(t *testing.T) {
	m := New(&Config{Issuer: "test"})
	_, err := m.GenerateKey(context.Background(), "", "test")
	if err == nil {
		t.Fatal("expected error for empty account name")
	}
}

func TestGenerateKey_Success(t *testing.T) {
	m := New(&Config{Issuer: "MyApp", Digits: 6, Period: 30})
	key, err := m.GenerateKey(context.Background(), "user@example.com", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key.Secret == "" {
		t.Fatal("expected non-empty secret")
	}
	if !strings.HasPrefix(key.URL, "otpauth://totp/") {
		t.Fatalf("unexpected URL: %s", key.URL)
	}
}

func TestGenerateQRCode_Success(t *testing.T) {
	m := New(nil)
	key, err := m.GenerateKey(context.Background(), "user@example.com", "MyApp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	png, err := m.GenerateQRCode(context.Background(), key, 256)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(png) == 0 {
		t.Fatal("expected non-empty PNG")
	}
}

func TestValidateCode_Success(t *testing.T) {
	m := New(&Config{Digits: 6, Period: 30})
	key, err := m.GenerateKey(context.Background(), "user@example.com", "MyApp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	code, err := totp.GenerateCode(key.Secret, time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error generating code: %v", err)
	}

	if !m.ValidateCode(context.Background(), key.Secret, code, 1) {
		t.Fatalf("expected code %q to be valid", code)
	}
}

func TestValidateCode_Invalid(t *testing.T) {
	m := New(nil)
	key, err := m.GenerateKey(context.Background(), "user@example.com", "MyApp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ValidateCode(context.Background(), key.Secret, "000000", 1) {
		t.Fatal("expected invalid code")
	}
}

func TestGenerateRecoveryCodes(t *testing.T) {
	m := New(nil)
	plain, hashed, err := m.GenerateRecoveryCodes(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plain) != 5 || len(hashed) != 5 {
		t.Fatalf("expected 5 codes, got %d/%d", len(plain), len(hashed))
	}
	for i, p := range plain {
		if p == "" {
			t.Fatalf("plain code %d is empty", i)
		}
		if hashed[i] == "" {
			t.Fatalf("hashed code %d is empty", i)
		}
		if strings.Contains(p, hashed[i]) {
			t.Fatal("hash should not contain plain code")
		}
	}
}

func TestValidateRecoveryCode_Success(t *testing.T) {
	m := New(nil)
	plain, hashed, err := m.GenerateRecoveryCodes(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ok, idx := m.ValidateRecoveryCode(context.Background(), plain[1], hashed)
	if !ok {
		t.Fatal("expected recovery code to match")
	}
	if idx != 1 {
		t.Fatalf("expected index 1, got %d", idx)
	}
}

func TestValidateRecoveryCode_Failure(t *testing.T) {
	m := New(nil)
	_, hashed, err := m.GenerateRecoveryCodes(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ok, idx := m.ValidateRecoveryCode(context.Background(), "AAAA-BBBB-CCCC-DDDD", hashed)
	if ok || idx != -1 {
		t.Fatal("expected no match")
	}
}

func TestValidateCode_EmptyInputs(t *testing.T) {
	m := New(nil)
	if m.ValidateCode(context.Background(), "", "123456", 1) {
		t.Fatal("expected empty secret to fail")
	}
	if m.ValidateCode(context.Background(), "secret", "", 1) {
		t.Fatal("expected empty code to fail")
	}
}

// fakeTwoFA is a minimal implementation used to verify interface shape.
type fakeTwoFA struct{}

func (f *fakeTwoFA) GenerateKey(ctx context.Context, accountName, issuer string) (*Key, error) {
	return &Key{}, nil
}
func (f *fakeTwoFA) GenerateQRCode(ctx context.Context, key *Key, size int) ([]byte, error) {
	return nil, nil
}
func (f *fakeTwoFA) ValidateCode(ctx context.Context, secret, code string, skew uint) bool {
	return false
}
func (f *fakeTwoFA) GenerateRecoveryCodes(ctx context.Context, count int) ([]string, []string, error) {
	return nil, nil, nil
}
func (f *fakeTwoFA) ValidateRecoveryCode(ctx context.Context, code string, hashedCodes []string) (bool, int) {
	return false, -1
}

func TestTwoFAInterface_Compliance(t *testing.T) {
	var _ TwoFA = (*fakeTwoFA)(nil)
}
