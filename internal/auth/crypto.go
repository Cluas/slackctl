package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// DecryptChromiumCookie decrypts a Chromium cookie value using PBKDF2 + AES-128-CBC.
func DecryptChromiumCookie(data []byte, password string, iterations int) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	if iterations < 1 {
		return "", fmt.Errorf("iterations must be >= 1, got %d", iterations)
	}

	salt := []byte("saltysalt")
	iv := []byte("                ") // 16 spaces
	key := pbkdf2.Key([]byte(password), salt, iterations, 16, sha1.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(data)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(data))
	mode.CryptBlocks(plain, data)

	// Remove PKCS7 padding
	if len(plain) > 0 {
		padLen := int(plain[len(plain)-1])
		if padLen > 0 && padLen <= aes.BlockSize && padLen <= len(plain) {
			plain = plain[:len(plain)-padLen]
		}
	}

	// Find xoxd- token in decrypted text
	xoxdRe := regexp.MustCompile(`xoxd-[A-Za-z0-9%/+_=.-]+`)
	match := xoxdRe.FindString(string(plain))
	if match != "" {
		return match, nil
	}
	return string(plain), nil
}

// GetSafeStoragePasswords retrieves Chromium Safe Storage passwords from the system keychain.
func GetSafeStoragePasswords(prefix string) []string {
	passwords := getSafeStoragePasswordsOS(prefix)
	// Deduplicate
	seen := make(map[string]bool)
	var unique []string
	for _, p := range passwords {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}
	return unique
}

// DecryptCookieWindows decrypts a Chromium cookie on Windows using DPAPI + AES-256-GCM.
// This is a placeholder — Windows support requires os/exec PowerShell calls.
func DecryptCookieWindows(encrypted []byte, slackDataDir string) (string, error) {
	_ = slackDataDir
	_ = encrypted
	return "", fmt.Errorf("Windows DPAPI decryption not yet implemented in Go version")
}

// findXoxdInDecrypted extracts xoxd cookie from raw decrypted string.
func findXoxdInDecrypted(s string) string {
	idx := strings.Index(s, "xoxd-")
	if idx == -1 {
		return s
	}
	end := idx
	for end < len(s) {
		b := s[end]
		if b < 0x21 || b > 0x7e {
			break
		}
		end++
	}
	return s[idx:end]
}
