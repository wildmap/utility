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

// 加密工具包
//
// 提供企业级的对称与非对称加密解决方案，所有实现均遵循当前密码学最佳实践。
//
// 支持的加密算法：
//  1. RSA-OAEP：非对称加密，适合密钥交换和小数据加密（≤190字节/2048位密钥）
//  2. AES-GCM：认证加密（推荐），同时保证机密性与完整性，防篡改
//  3. AES-CTR：流式对称加密，适合大数据量但不提供认证，需配合 HMAC 使用
//
// 选型建议：
//   - 密钥交换 / 小数据加密：RSA-OAEP
//   - 通用数据加密：AES-GCM（首选）
//   - 大数据流式加密：AES-CTR + HMAC-SHA256
//
// 注意事项：
//   - AES 密钥须通过安全渠道分发，泄露将导致数据完全暴露
//   - 生产环境建议结合硬件安全模块（HSM）或密钥管理服务（KMS）管理私钥

// 加密操作相关的预定义错误。
var (
	// ErrInvalidPEMBlock 表示 PEM 格式解析失败，通常由文件损坏或格式不正确导致。
	ErrInvalidPEMBlock = errors.New("invalid PEM block")

	// ErrInvalidPrivateKey 表示私钥类型不匹配，非 RSA 私钥或密钥格式错误时触发。
	ErrInvalidPrivateKey = errors.New("invalid private key type")

	// ErrInvalidPublicKey 表示公钥类型不匹配，非 RSA 公钥或密钥格式错误时触发。
	ErrInvalidPublicKey = errors.New("invalid public key type")

	// ErrInvalidAESKeyLength 表示 AES 密钥长度非法。
	// 合法长度：16 字节（AES-128）、24 字节（AES-192）、32 字节（AES-256）。
	ErrInvalidAESKeyLength = errors.New("AES key length must be 16, 24, or 32 bytes")

	// ErrInvalidCiphertext 表示密文格式错误或数据被截断/篡改。
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
)

// ICrypter 定义统一的加密解密接口。
//
// 所有加密实现（RSA、AES-GCM、AES-CTR）均实现此接口，
// 通过面向接口编程可在运行时灵活切换加密算法，降低业务代码与具体算法的耦合。
type ICrypter interface {
	// Encrypt 加密明文数据，返回密文字节数组。
	// AES 系列算法的密文中已内嵌 nonce/IV，解密时无需额外传递。
	Encrypt(data []byte) ([]byte, error)

	// Decrypt 解密密文数据，返回明文字节数组。
	// AES-GCM 解密会同时验证认证标签，若标签校验失败则返回错误，表明数据可能已被篡改。
	Decrypt(data []byte) ([]byte, error)
}

// ==================== RSA 非对称加密 ====================

// RSADecrypter 使用 RSA 私钥和 OAEP 填充方案解密数据。
//
// OAEP（Optimal Asymmetric Encryption Padding）相比传统 PKCS#1 v1.5 填充
// 能有效抵御选择密文攻击，是当前 RSA 加密的推荐填充方案。
// 内部采用 SHA-256 作为哈希函数，提供更强的安全性。
//
// 密钥格式支持：
//   - PKCS#8（推荐）：跨语言兼容性好，适合现代系统
//   - PKCS#1（兼容旧系统）：创建时自动回退尝试
type RSADecrypter struct {
	privateKey *rsa.PrivateKey // RSA 私钥，持有者须保证其安全性
}

// NewRSADecrypter 解析 PEM 格式的 RSA 私钥并创建解密器实例。
//
// 解析策略：优先尝试 PKCS#8 格式，失败后自动回退至 PKCS#1 格式，
// 兼容新旧两种密钥格式，无需调用方手动区分。
func NewRSADecrypter(priKey []byte) (*RSADecrypter, error) {
	// 解码 PEM 封装层，提取 DER 编码的密钥数据
	block, _ := pem.Decode(priKey)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}

	// 优先尝试 PKCS#8 格式解析，失败后回退至 PKCS#1
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, ErrInvalidPrivateKey
		}
	}

	// 类型断言确保密钥为 RSA 类型，防止将 EC 等其他类型密钥误用
	privateKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidPrivateKey
	}

	return &RSADecrypter{privateKey: privateKey}, nil
}

// Decrypt 使用 RSA-OAEP（SHA-256）解密数据。
//
// RSA 解密属于 CPU 密集型操作，2048 位密钥单次解密约耗时 1~2ms，
// 不适合高频或大数据场景，通常仅用于解密对称密钥本身（混合加密方案）。
func (d *RSADecrypter) Decrypt(encryptedData []byte) ([]byte, error) {
	if len(encryptedData) == 0 {
		return nil, errors.New("加密数据为空")
	}

	// rsa.DecryptOAEP 内部使用 rand.Reader 进行盲化操作，防止计时攻击
	hash := sha256.New()
	originData, err := rsa.DecryptOAEP(hash, rand.Reader, d.privateKey, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA解密失败: %w", err)
	}
	return originData, nil
}

// RSAEncrypter 使用 RSA 公钥和 OAEP 填充方案加密数据。
//
// 公钥加密，私钥解密——任何持有公钥的一方均可加密，
// 但只有持有对应私钥的一方才能解密，适合安全传递对称密钥等场景。
type RSAEncrypter struct {
	publicKey *rsa.PublicKey // RSA 公钥，可公开分发
}

// NewRSAEncrypter 解析 PKIX 格式的 PEM 公钥并创建加密器实例。
func NewRSAEncrypter(pubKey []byte) (*RSAEncrypter, error) {
	block, _ := pem.Decode(pubKey)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}

	// PKIX 是 X.509 标准定义的公钥格式，兼容 openssl 生成的 RSA 公钥
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析PKIX公钥失败: %w", err)
	}

	publicKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return nil, ErrInvalidPublicKey
	}

	return &RSAEncrypter{publicKey: publicKey}, nil
}

// Encrypt 使用 RSA-OAEP（SHA-256）加密数据。
//
// 每次加密使用 rand.Reader 引入随机性，相同明文每次产生不同密文（概率加密），
// 有效防止密文分析攻击。
func (e *RSAEncrypter) Encrypt(originData []byte) ([]byte, error) {
	if len(originData) == 0 {
		return nil, errors.New("原始数据为空")
	}

	hash := sha256.New()
	encryptedData, err := rsa.EncryptOAEP(hash, rand.Reader, e.publicKey, originData, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA加密失败: %w", err)
	}
	return encryptedData, nil
}

// ==================== AES-GCM 认证加密 ====================

// AESGCMCrypter 实现 AES-GCM（Galois/Counter Mode）认证加密。
//
// GCM 模式是 CTR 模式与 GHASH 认证的组合，在提供加密机密性的同时，
// 通过 128 位认证标签（Authentication Tag）保证数据完整性，
// 任何对密文的篡改都会导致解密失败——这是区别于 AES-CTR 的核心优势。
// 推荐在绝大多数场景下优先使用此模式。
type AESGCMCrypter struct {
	aead cipher.AEAD // AEAD（Authenticated Encryption with Associated Data）接口封装
}

// NewAESGCMCrypter 创建 AES-GCM 加密器实例。
//
// 密钥长度决定 AES 安全级别：
//   - 16 字节 → AES-128（足以应对当前已知威胁）
//   - 24 字节 → AES-192
//   - 32 字节 → AES-256（政府机密级别）
func NewAESGCMCrypter(key []byte) (*AESGCMCrypter, error) {
	keyLen := len(key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, ErrInvalidAESKeyLength
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建AES密码块失败: %w", err)
	}

	// 基于 AES 块密码构建 GCM 模式，默认 nonce 大小为 12 字节
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM失败: %w", err)
	}

	return &AESGCMCrypter{aead: aead}, nil
}

// Encrypt 使用 AES-GCM 加密明文，密文格式为 [nonce | 密文 | 认证标签]。
//
// 每次加密通过 io.ReadFull 从 rand.Reader 生成唯一随机 nonce（12字节），
// 将 nonce 前置于密文便于解密时提取，无需额外传递 nonce。
// 注意：同一密钥下绝对不能重复使用 nonce，否则将完全破坏安全性。
func (c *AESGCMCrypter) Encrypt(plaintext []byte) ([]byte, error) {
	// 生成密码学安全的随机 nonce（number used once），GCM 标准 nonce 长度为 12 字节
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成nonce失败: %w", err)
	}

	// Seal 在 nonce 后追加密文和认证标签，返回 [nonce | ciphertext | tag]
	ciphertext := c.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt 使用 AES-GCM 解密密文并验证认证标签。
//
// 若认证标签验证失败（数据被篡改或密钥不匹配），解密将返回错误而非静默失败，
// 这是 GCM 模式相比纯 CTR 模式的核心安全优势。
func (c *AESGCMCrypter) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()
	// 密文至少包含 nonce（12字节），长度不足则必然是损坏的数据
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	// 从密文头部切分出 nonce 和真正的加密数据
	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Open 同时解密并验证认证标签，任何篡改都会导致此步骤失败
	plaintext, err := c.aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("AES-GCM解密失败: %w", err)
	}
	return plaintext, nil
}

// ==================== AES-CTR 流式加密 ====================

// AESCTRCrypter 实现 AES-CTR（Counter Mode）流式加密。
//
// CTR 模式通过对自增计数器加密生成密钥流，再与明文异或得到密文，
// 将块密码转换为流密码，支持任意长度数据的并行加解密。
//
// 重要：CTR 模式本身不提供数据完整性保护，若需防篡改，
// 应在加密后追加 HMAC-SHA256 认证，或直接使用 AES-GCM 模式。
type AESCTRCrypter struct {
	key []byte // AES 密钥，长度 16/24/32 字节
}

// NewAESCTRCrypter 创建 AES-CTR 加密器实例。
func NewAESCTRCrypter(key []byte) (*AESCTRCrypter, error) {
	keyLen := len(key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, ErrInvalidAESKeyLength
	}

	return &AESCTRCrypter{key: key}, nil
}

// Encrypt 使用 AES-CTR 模式加密明文，密文格式为 [IV | 密文]。
//
// 每次加密生成随机 IV（初始化向量，16字节，与 AES 块大小相同）并前置于密文。
// CTR 模式的每次加密都应使用唯一的 IV，否则相同密钥下相同 IV 的密文可被异或破解。
func (c *AESCTRCrypter) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("创建AES密码块失败: %w", err)
	}

	// 预分配 [IV空间 | 密文空间] 的连续内存，减少内存分配次数
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize] // IV 占据密文前 16 字节

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("生成IV失败: %w", err)
	}

	// NewCTR 基于 IV 初始化计数器，XORKeyStream 对明文进行流式加密
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

// Decrypt 使用 AES-CTR 模式解密密文。
//
// CTR 模式的加解密操作完全对称（均为 XOR 操作），
// 解密过程与加密过程完全相同，只需正确提取 IV 并重建相同的密钥流即可。
func (c *AESCTRCrypter) Decrypt(ciphertext []byte) ([]byte, error) {
	// 密文必须至少包含 IV（aes.BlockSize = 16 字节）
	if len(ciphertext) < aes.BlockSize {
		return nil, ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("创建AES密码块失败: %w", err)
	}

	// 切分 IV 与实际加密数据
	iv := ciphertext[:aes.BlockSize]
	encrypted := ciphertext[aes.BlockSize:]

	// CTR 解密与加密完全对称，均为 XOR 密钥流操作
	plaintext := make([]byte, len(encrypted))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, encrypted)

	return plaintext, nil
}
