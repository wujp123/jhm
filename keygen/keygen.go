package keygen

import (
	"bytes"
	"compress/gzip"
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

const privateKeyPath = "private.pem"

type LicenseData struct {
	MachineID string `json:"machine_id"`
	ExpiryUTC int64  `json:"expiry_utc"`
}

type License struct {
	Data      string `json:"data"`
	Signature string `json:"signature"`
}

func GenerateKeyPair() error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(privateKeyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

func GenerateLicense(machineID, expiry string) (string, error) {
	if machineID == "" || expiry == "" {
		return "", errors.New("missing field")
	}

	t, err := time.Parse("2006-01-02", expiry)
	if err != nil {
		return "", err
	}

	data, _ := json.Marshal(LicenseData{
		MachineID: machineID,
		ExpiryUTC: t.Add(24*time.Hour - time.Second).UTC().Unix(),
	})

	privPem, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return "", err
	}

	block, _ := pem.Decode(privPem)
	if block == nil {
		return "", errors.New("bad private key")
	}

	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}

	lic, _ := json.Marshal(License{
		Data:      base64.StdEncoding.EncodeToString(data),
		Signature: base64.StdEncoding.EncodeToString(sig),
	})

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write(lic)
	gz.Close()

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}