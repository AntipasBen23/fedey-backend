package tokens

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
)

type Cipher interface {
	Encrypt(value string) (string, error)
	Decrypt(value string) (string, error)
}

type NoopCipher struct{}

func NewNoopCipher() NoopCipher {
	return NoopCipher{}
}

func (NoopCipher) Encrypt(value string) (string, error) {
	return value, nil
}

func (NoopCipher) Decrypt(value string) (string, error) {
	return value, nil
}

type AESCipher struct {
	gcm cipher.AEAD
}

func NewAESCipher(base64Key string) (*AESCipher, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(base64Key))
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must decode to 32 bytes")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm cipher: %w", err)
	}

	return &AESCipher{gcm: gcm}, nil
}

func (c *AESCipher) Encrypt(value string) (string, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	sealed := c.gcm.Seal(nonce, nonce, []byte(value), nil)
	return "enc:" + base64.StdEncoding.EncodeToString(sealed), nil
}

func (c *AESCipher) Decrypt(value string) (string, error) {
	if !strings.HasPrefix(value, "enc:") {
		return value, nil
	}

	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "enc:"))
	if err != nil {
		return "", fmt.Errorf("decode encrypted payload: %w", err)
	}
	if len(payload) < c.gcm.NonceSize() {
		return "", fmt.Errorf("encrypted payload too short")
	}

	nonce := payload[:c.gcm.NonceSize()]
	ciphertext := payload[c.gcm.NonceSize():]
	plaintext, err := c.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt token: %w", err)
	}

	return string(plaintext), nil
}
