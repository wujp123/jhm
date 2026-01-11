package keygen

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"time"
)

type LicensePayload struct {
	MachineID string `json:"machine_id"`
	Expiry    int64  `json:"expiry"`
}

// 生成 RSA 密钥对（若不存在）
func GenerateKeyPair() error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	privBytes := x509.MarshalPKCS1PrivateKey(key)
	pubBytes := x509.MarshalPKCS1PublicKey(&key.PublicKey)

	if err := writePem("backend/private.pem", "RSA PRIVATE KEY", privBytes); err != nil {
		return err
	}
	if err := writePem("backend/public.pem", "RSA PUBLIC KEY", pubBytes); err != nil {
		return err
	}
	return nil
}

func writePem(path, t string, b []byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return pem.Encode(f, &pem.Block{
		Type:  t,
		Bytes: b,
	})
}

// 生成激活码
func GenerateLicense(machineID, expiry string) (string, error) {
	if machineID == "" {
		return "", errors.New("machine_id empty")
	}
	if expiry == "" {
		return "", errors.New("expiry empty")
	}

	// YYYY-MM-DD
	t, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return "", err
	}

	payload := LicensePayload{
		MachineID: machineID,
		Expiry:   t.Unix(),
	}

	data, _ := json.Marshal(payload)

	// SHA256
	hash := sha256.Sum256(data)

	// 读取私钥
	privPem, err := os.ReadFile("backend/private.pem")
	if err != nil {
		return "", err
	}

	block, _ := pem.Decode(privPem)
	if block == nil {
		return "", errors.New("invalid private key")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	// ✅ 正确的签名方式
	signature, err := rsa.SignPKCS1v15(
		rand.Reader,
		privKey,
		crypto.SHA256,
		hash[:],
	)
	if err != nil {
		return "", err
	}

	license := map[string]string{
		"data": base64.StdEncoding.EncodeToString(data),
		"sig":  base64.StdEncoding.EncodeToString(signature),
	}

	out, _ := json.Marshal(license)
	return base64.StdEncoding.EncodeToString(out), nil
}