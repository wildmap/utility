package utility

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
)

// Error definitions for cryptographic operations
var (
	ErrInvalidPEMBlock     = errors.New("invalid PEM block")
	ErrInvalidPrivateKey   = errors.New("invalid private key type")
	ErrInvalidPublicKey    = errors.New("invalid public key type")
	ErrInvalidAESKeyLength = errors.New("AES key length must be 16, 24, or 32 bytes")
	ErrInvalidCiphertext   = errors.New("invalid ciphertext")
)

// ICrypter defines the interface for encryption and decryption operations
type ICrypter interface {
	// Encrypt encrypts the given data
	Encrypt(data []byte) ([]byte, error)
	// Decrypt decrypts the given data
	Decrypt(data []byte) ([]byte, error)
}

// ==================== RSA Encryption/Decryption ====================

// RSADecrypter implements RSA decryption with OAEP padding
type RSADecrypter struct {
	privateKey *rsa.PrivateKey
}

// NewRSADecrypter creates a new RSA decrypter instance
func NewRSADecrypter(priKey []byte) (*RSADecrypter, error) {
	// Decode PEM block
	block, _ := pem.Decode(priKey)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}

	// Try parsing as PKCS8 first, then fall back to PKCS1
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, ErrInvalidPrivateKey
		}
	}

	// Type assert to RSA private key
	privateKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidPrivateKey
	}

	return &RSADecrypter{privateKey: privateKey}, nil
}

// Decrypt decrypts data using RSA-OAEP with SHA-256 hash function
func (d *RSADecrypter) Decrypt(encryptedData []byte) ([]byte, error) {
	if len(encryptedData) == 0 {
		return nil, errors.New("encrypted data is empty")
	}

	// Use SHA-256 hash for OAEP padding
	hash := sha256.New()
	originData, err := rsa.DecryptOAEP(hash, rand.Reader, d.privateKey, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA decrypt failed: %w", err)
	}
	return originData, nil
}

// RSAEncrypter implements RSA encryption with OAEP padding
type RSAEncrypter struct {
	publicKey *rsa.PublicKey
}

// NewRSAEncrypter creates a new RSA encrypter instance
func NewRSAEncrypter(pubKey []byte) (*RSAEncrypter, error) {
	// Decode PEM block
	block, _ := pem.Decode(pubKey)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}

	// Parse PKIX public key
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKIX public key failed: %w", err)
	}

	// Type assert to RSA public key
	publicKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return nil, ErrInvalidPublicKey
	}

	return &RSAEncrypter{publicKey: publicKey}, nil
}

// Encrypt encrypts data using RSA-OAEP with SHA-256 hash function
func (e *RSAEncrypter) Encrypt(originData []byte) ([]byte, error) {
	if len(originData) == 0 {
		return nil, errors.New("origin data is empty")
	}

	// Use SHA-256 hash for OAEP padding
	hash := sha256.New()
	encryptedData, err := rsa.EncryptOAEP(hash, rand.Reader, e.publicKey, originData, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA encrypt failed: %w", err)
	}
	return encryptedData, nil
}

// ==================== AES-GCM Encryption/Decryption ====================

// AESGCMCrypter implements authenticated encryption using AES-GCM mode
// This is the recommended method as it provides both confidentiality and authenticity
type AESGCMCrypter struct {
	aead cipher.AEAD
}

// NewAESGCMCrypter creates a new AES-GCM crypter instance
func NewAESGCMCrypter(key []byte) (*AESGCMCrypter, error) {
	keyLen := len(key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, ErrInvalidAESKeyLength
	}

	// Create AES cipher block
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher failed: %w", err)
	}

	// Create GCM mode wrapper
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM failed: %w", err)
	}

	return &AESGCMCrypter{aead: aead}, nil
}

// Encrypt encrypts plaintext using AES-GCM
func (c *AESGCMCrypter) Encrypt(plaintext []byte) ([]byte, error) {
	// Generate random nonce
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce failed: %w", err)
	}

	// Seal appends the ciphertext and tag to the nonce
	ciphertext := c.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-GCM and verifies the authentication tag
func (c *AESGCMCrypter) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	// Extract nonce and encrypted data
	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt and verify authentication tag
	plaintext, err := c.aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("AES-GCM decrypt failed: %w", err)
	}
	return plaintext, nil
}

// ==================== AES-CTR Mode Encryption/Decryption ====================

// AESCTRCrypter implements AES encryption in CTR (Counter) mode
// Note: CTR mode does not provide authentication. For authenticated encryption, use AES-GCM instead
type AESCTRCrypter struct {
	key []byte
}

// NewAESCTRCrypter creates a new AES-CTR crypter instance
func NewAESCTRCrypter(key []byte) (*AESCTRCrypter, error) {
	keyLen := len(key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, ErrInvalidAESKeyLength
	}

	return &AESCTRCrypter{key: key}, nil
}

// Encrypt encrypts plaintext using AES-CTR mode
func (c *AESCTRCrypter) Encrypt(plaintext []byte) ([]byte, error) {
	// Create AES cipher block
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher failed: %w", err)
	}

	// Allocate space for IV and ciphertext
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]

	// Generate random IV
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("generate IV failed: %w", err)
	}

	// Create CTR mode stream and encrypt
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-CTR mode
func (c *AESCTRCrypter) Decrypt(ciphertext []byte) ([]byte, error) {
	// Validate minimum ciphertext length
	if len(ciphertext) < aes.BlockSize {
		return nil, ErrInvalidCiphertext
	}

	// Create AES cipher block
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher failed: %w", err)
	}

	// Extract IV and encrypted data
	iv := ciphertext[:aes.BlockSize]
	encrypted := ciphertext[aes.BlockSize:]

	// Decrypt using CTR mode
	plaintext := make([]byte, len(encrypted))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, encrypted)

	return plaintext, nil
}

