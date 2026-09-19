// Package secret 提供 AES-256-GCM 凭据静态加密（architecture §3 安全列说明）。
// 主密钥 32 字节；密文格式：nonce(12B) || ciphertext+tag。解密失败返回错误（不重试）。
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Cipher 持有 AES-GCM 实例。
type Cipher struct {
	aead cipher.AEAD
}

var (
	// ErrNoMasterKey 表示未配置主密钥（凭据写入应拒绝服务）。
	ErrNoMasterKey = errors.New("secret: master key 未配置")
	// ErrInvalidKeyLen 主密钥必须 32 字节。
	ErrInvalidKeyLen = errors.New("secret: master key 必须 32 字节（AES-256）")
)

// NewCipher 用 32 字节主密钥构建。
func NewCipher(masterKey []byte) (*Cipher, error) {
	if len(masterKey) == 0 {
		return nil, ErrNoMasterKey
	}
	if len(masterKey) != 32 {
		return nil, ErrInvalidKeyLen
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("secret: 创建 cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secret: 创建 GCM: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt 明文 → 密文（nonce || ciphertext）。
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("secret: 生成 nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt 密文 → 明文。
func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("secret: 密文过短")
	}
	plaintext, err := c.aead.Open(nil, ciphertext[:ns], ciphertext[ns:], nil)
	if err != nil {
		return nil, fmt.Errorf("secret: 解密失败（密钥不匹配或密文损坏）: %w", err)
	}
	return plaintext, nil
}

// DecryptString 便捷字符串解密。
func (c *Cipher) DecryptString(s string) (string, error) {
	b, err := c.Decrypt([]byte(s))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// GenerateMasterKey 生成 base64 编码的随机 32 字节主密钥（doctor/引导用）。
func GenerateMasterKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("secret: 生成随机密钥: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}
