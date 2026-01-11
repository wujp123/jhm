package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

const (
	privateKeyPath = "keygen-keys/private.pem"
	publicKeyPath  = "backend/public.pem"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "generate" {
		fmt.Println("用法: go run keygen/cmd/main.go generate")
		return
	}

	fmt.Println("🔐 正在生成 RSA 密钥对...")

	if err := os.MkdirAll(filepath.Dir(privateKeyPath), 0700); err != nil {
		panic(err)
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	// 私钥
	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privFile, _ := os.OpenFile(privateKeyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	pem.Encode(privFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes})
	privFile.Close()

	// 公钥
	pubBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	pubFile, _ := os.Create(publicKeyPath)
	pem.Encode(pubFile, &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	pubFile.Close()

	fmt.Println("✅ 密钥生成完成")
	fmt.Println("私钥:", privateKeyPath)
	fmt.Println("公钥:", publicKeyPath)
	fmt.Println("⚠️ 私钥不要提交到 GitHub")
}