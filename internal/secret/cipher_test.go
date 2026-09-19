package secret

import (
	"testing"
)

func TestCipherRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	c, err := NewCipher(key)
	if err != nil {
		t.Fatalf("NewCipher: %v", err)
	}

	plaintexts := []string{
		"",
		"hunter2",
		"sk-abc123def456",
		"包含中文与符号!@#$%^&*()",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature-long-token",
	}
	for _, pt := range plaintexts {
		ct, err := c.Encrypt([]byte(pt))
		if err != nil {
			t.Fatalf("Encrypt(%q): %v", pt, err)
		}
		if len(ct) > 0 && string(ct) == pt {
			t.Fatalf("密文不应等于明文")
		}
		got, err := c.DecryptString(string(ct))
		if err != nil {
			t.Fatalf("Decrypt(%q): %v", pt, err)
		}
		if got != pt {
			t.Fatalf("roundtrip 失败: got %q want %q", got, pt)
		}
	}
}

func TestCipherWrongKeyFails(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1
	c1, _ := NewCipher(key1)
	c2, _ := NewCipher(key2)

	ct, err := c1.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if _, err := c2.Decrypt(ct); err == nil {
		t.Fatal("用错误密钥解密应当失败")
	}
}

func TestCipherKeyValidation(t *testing.T) {
	if _, err := NewCipher(nil); err != ErrNoMasterKey {
		t.Fatalf("空密钥应返回 ErrNoMasterKey, got %v", err)
	}
	if _, err := NewCipher(make([]byte, 16)); err != ErrInvalidKeyLen {
		t.Fatalf("16 字节密钥应返回 ErrInvalidKeyLen, got %v", err)
	}
	if _, err := NewCipher(make([]byte, 32)); err != nil {
		t.Fatalf("32 字节密钥应成功: %v", err)
	}
}

func TestGenerateMasterKey(t *testing.T) {
	k1, err := GenerateMasterKey()
	if err != nil {
		t.Fatalf("GenerateMasterKey: %v", err)
	}
	k2, _ := GenerateMasterKey()
	if k1 == k2 {
		t.Fatal("两次生成的密钥不应相同")
	}
	if len(k1) == 0 {
		t.Fatal("密钥不应为空")
	}
}
