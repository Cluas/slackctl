package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"testing"

	"golang.org/x/crypto/pbkdf2"
)

func TestDecryptChromiumCookie(t *testing.T) {
	// Encrypt a known value, then decrypt it
	password := "testpassword"
	iterations := 1003
	salt := []byte("saltysalt")
	iv := []byte("                ") // 16 spaces
	key := pbkdf2.Key([]byte(password), salt, iterations, 16, sha1.New)

	plaintext := []byte("xoxd-test-cookie-value")
	// PKCS7 pad
	padLen := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)

	// Decrypt
	got, err := DecryptChromiumCookie(ciphertext, password, iterations)
	if err != nil {
		t.Fatalf("DecryptChromiumCookie: %v", err)
	}
	if got != "xoxd-test-cookie-value" {
		t.Errorf("got %q, want %q", got, "xoxd-test-cookie-value")
	}
}

func TestDecryptChromiumCookie_Empty(t *testing.T) {
	got, err := DecryptChromiumCookie(nil, "pw", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestDecryptChromiumCookie_BadIterations(t *testing.T) {
	_, err := DecryptChromiumCookie([]byte("data"), "pw", 0)
	if err == nil {
		t.Fatal("expected error for iterations=0")
	}
}
