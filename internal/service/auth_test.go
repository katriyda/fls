package service

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestGenerateSessionToken_Length(t *testing.T) {
	token, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken() returned error: %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("expected token length 64, got %d", len(token))
	}
}

func TestGenerateSessionToken_HexEncoding(t *testing.T) {
	token, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken() returned error: %v", err)
	}
	for _, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Fatalf("token contains non-hex character: %c", c)
		}
	}
}

func TestGenerateSessionToken_Unique(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, err := GenerateSessionToken()
		if err != nil {
			t.Fatalf("GenerateSessionToken() returned error: %v", err)
		}
		if tokens[token] {
			t.Fatal("duplicate token generated")
		}
		tokens[token] = true
	}
}

func TestGenerateSessionToken_Entropy(t *testing.T) {
	tokens := make([]string, 10)
	for i := range tokens {
		var err error
		tokens[i], err = GenerateSessionToken()
		if err != nil {
			t.Fatalf("GenerateSessionToken() returned error: %v", err)
		}
	}
	for i := 1; i < len(tokens); i++ {
		if tokens[i] == tokens[i-1] {
			t.Fatal("consecutive tokens should not be equal")
		}
	}
}

func TestVerifyPassword_Valid(t *testing.T) {
	password := "test-password-123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	err = bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil {
		t.Fatal("expected valid password to pass verification")
	}
}

func TestVerifyPassword_Invalid(t *testing.T) {
	password := "test-password-123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	err = bcrypt.CompareHashAndPassword(hash, []byte("wrong-password"))
	if err == nil {
		t.Fatal("expected invalid password to fail verification")
	}
}

func TestVerifyPassword_Empty(t *testing.T) {
	password := "test-password-123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	err = bcrypt.CompareHashAndPassword(hash, []byte(""))
	if err == nil {
		t.Fatal("expected empty password to fail verification")
	}
}

func TestVerifyPassword_DifferentPasswords(t *testing.T) {
	passwords := []string{"alpha", "beta", "gamma", "delta"}
	hashes := make([][]byte, len(passwords))
	for i, p := range passwords {
		h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
		if err != nil {
			t.Fatalf("failed to hash password %q: %v", p, err)
		}
		hashes[i] = h
	}

	for i, p := range passwords {
		err := bcrypt.CompareHashAndPassword(hashes[i], []byte(p))
		if err != nil {
			t.Errorf("expected password %q to match its own hash", p)
		}
		for j := range passwords {
			if i == j {
				continue
			}
			err := bcrypt.CompareHashAndPassword(hashes[i], []byte(passwords[j]))
			if err == nil {
				t.Errorf("password %q should not match hash of %q", passwords[j], passwords[i])
			}
		}
	}
}

func TestVerifyPassword_SpecialCharacters(t *testing.T) {
	password := "p@ssw0rd! #$%^&*()_+=-[]{}|;:',.<>?/~`"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password with special chars: %v", err)
	}

	err = bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil {
		t.Fatal("expected password with special chars to pass verification")
	}
}
