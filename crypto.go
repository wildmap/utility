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
// 提供了企业级的加密解密解决方案，支持对称加密和非对称加密。
//
// 支持的加密算法：
//   1. RSA-OAEP：非对称加密，适合密钥交换和小数据加密
//   2. AES-GCM：认证加密（推荐），提供机密性和完整性双重保护
//   3. AES-CTR：流式加密，适合大数据加密但不提供认证
//
// 安全特性：
//   - 使用 SHA-256 作为哈希函数
//   - 自动生成随机 nonce/IV
//   - AES-GCM 提供防篡改保护
//   - 支持 PKCS1 和 PKCS8 密钥格式
//
// 推荐使用方案：
//   - 小数据/密钥交换：RSA-OAEP
//   - 一般数据加密：AES-GCM（推荐）
//   - 大数据流式加密：AES-CTR（需额外认证）
//
// 注意事项：
//   - RSA 加密数据大小受密钥长度限制（2048位密钥约190字节）
//   - AES 密钥必须妥善保管，泄露将导致数据完全暴露
//   - AES-CTR 模式不提供认证，需配合 HMAC 使用
//   - 生产环境建议使用硬件安全模块（HSM）管理密钥

// 加密操作相关错误定义
var (
	// ErrInvalidPEMBlock PEM 格式解析失败
	// 可能原因：文件不是 PEM 格式、文件损坏、格式错误
	ErrInvalidPEMBlock = errors.New("invalid PEM block")

	// ErrInvalidPrivateKey 私钥类型不匹配
	// 可能原因：非 RSA 私钥、密钥格式错误、密钥损坏
	ErrInvalidPrivateKey = errors.New("invalid private key type")

	// ErrInvalidPublicKey 公钥类型不匹配
	// 可能原因：非 RSA 公钥、密钥格式错误、密钥损坏
	ErrInvalidPublicKey = errors.New("invalid public key type")

	// ErrInvalidAESKeyLength AES 密钥长度不符合规范
	// 有效长度：16字节(AES-128)、24字节(AES-192)、32字节(AES-256)
	ErrInvalidAESKeyLength = errors.New("AES key length must be 16, 24, or 32 bytes")

	// ErrInvalidCiphertext 密文格式错误或数据损坏
	// 可能原因：密文被截断、密文被篡改、密文格式不匹配
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
)

// ICrypter 定义加密器的统一接口
//
// 所有加密实现（RSA、AES-GCM、AES-CTR）都实现此接口，
// 提供统一的加密解密方法，便于算法切换和模块化设计。
//
// 实现类：
//   - RSAEncrypter / RSADecrypter：非对称加密
//   - AESGCMCrypter：认证加密（推荐）
//   - AESCTRCrypter：流式加密
//
// 使用模式：
//
//	var crypter ICrypter
//	crypter = NewAESGCMCrypter(key)
//	ciphertext, err := crypter.Encrypt(plaintext)
//	plaintext, err := crypter.Decrypt(ciphertext)
type ICrypter interface {
	// Encrypt 加密明文数据
	//
	// 参数：
	//   data - 待加密的明文字节数组
	//
	// 返回值：
	//   []byte - 加密后的密文字节数组
	//   error  - 加密失败时的错误信息
	//
	// 注意：
	//   - RSA 加密有数据长度限制
	//   - AES 加密的密文长度会比明文长（包含 nonce/IV）
	Encrypt(data []byte) ([]byte, error)

	// Decrypt 解密密文数据
	//
	// 参数：
	//   data - 待解密的密文字节数组
	//
	// 返回值：
	//   []byte - 解密后的明文字节数组
	//   error  - 解密失败时的错误信息（密钥错误、密文损坏等）
	//
	// 注意：
	//   - 必须使用对应的密钥和算法进行解密
	//   - AES-GCM 解密时会验证认证标签，失败则返回错误
	Decrypt(data []byte) ([]byte, error)
}

// ==================== RSA 非对称加密/解密 ====================
//
// RSA 加密特点：
//   - 非对称加密：公钥加密，私钥解密
//   - 适用场景：密钥交换、数字签名、小数据加密
//   - 数据长度限制：受密钥长度限制，2048位密钥约可加密190字节
//   - 性能：相比对称加密慢，不适合大数据加密
//
// OAEP 填充方案：
//   - Optimal Asymmetric Encryption Padding
//   - 比传统 PKCS#1 v1.5 更安全
//   - 防止选择密文攻击
//   - 使用 SHA-256 作为哈希函数

// RSADecrypter RSA 解密器
//
// 使用 RSA 私钥和 OAEP 填充方案解密数据。
// 采用 SHA-256 哈希函数，提供更高的安全性。
//
// 密钥格式支持：
//   - PKCS#8：推荐格式，跨语言兼容性好
//   - PKCS#1：传统格式，主要用于兼容旧系统
//
// 使用示例：
//
//	decrypter, err := NewRSADecrypter(privateKeyPEM)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	plaintext, err := decrypter.Decrypt(ciphertext)
type RSADecrypter struct {
	privateKey *rsa.PrivateKey // RSA 私钥对象
}

// NewRSADecrypter 创建 RSA 解密器实例
//
// 自动识别并解析 PKCS#8 和 PKCS#1 两种密钥格式。
// 解析策略：优先尝试 PKCS#8，失败后回退到 PKCS#1。
//
// 参数：
//
//	priKey - PEM 格式的 RSA 私钥字节数组
//
// 返回值：
//
//	*RSADecrypter - RSA 解密器实例
//	error - 解析失败时返回错误
//
// 错误类型：
//   - ErrInvalidPEMBlock：PEM 格式解析失败
//   - ErrInvalidPrivateKey：密钥类型错误或格式不支持
//
// PEM 格式示例：
//
//	-----BEGIN PRIVATE KEY-----
//	MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIB...
//	-----END PRIVATE KEY-----
//
// 注意事项：
//   - 私钥必须严格保密，泄露将导致安全风险
//   - 生产环境建议使用密钥管理服务（KMS）
//   - 密钥应使用强密码保护
func NewRSADecrypter(priKey []byte) (*RSADecrypter, error) {
	// 第一步：解码 PEM 格式
	block, _ := pem.Decode(priKey)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}

	// 第二步：尝试解析密钥（优先 PKCS#8，回退 PKCS#1）
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// PKCS#8 解析失败，尝试 PKCS#1
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, ErrInvalidPrivateKey
		}
	}

	// 第三步：类型断言，确保是 RSA 私钥
	privateKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, ErrInvalidPrivateKey
	}

	return &RSADecrypter{privateKey: privateKey}, nil
}

// Decrypt 使用 RSA 私钥解密数据
//
// 解密算法：RSA-OAEP with SHA-256
// 填充方案：OAEP（Optimal Asymmetric Encryption Padding）
// 哈希函数：SHA-256
//
// 参数：
//
//	encryptedData - RSA 加密的密文字节数组
//
// 返回值：
//
//	[]byte - 解密后的明文
//	error - 解密失败时返回错误
//
// 常见错误：
//   - "加密数据为空"：输入数据长度为0
//   - "RSA解密失败"：密钥不匹配、数据损坏、填充验证失败
//
// 性能考虑：
//   - RSA 解密是 CPU 密集型操作
//   - 2048位密钥的解密耗时约 1-2ms
//   - 不适合频繁解密或大数据解密
//
// 安全注意：
//   - 确保密文来源可信，防止选择密文攻击
//   - 失败时不要泄露具体错误信息给用户
func (d *RSADecrypter) Decrypt(encryptedData []byte) ([]byte, error) {
	// 参数验证
	if len(encryptedData) == 0 {
		return nil, errors.New("加密数据为空")
	}

	// 使用 RSA-OAEP 解密（SHA-256 哈希）
	hash := sha256.New()
	originData, err := rsa.DecryptOAEP(hash, rand.Reader, d.privateKey, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA解密失败: %w", err)
	}
	return originData, nil
}

// RSAEncrypter 实现基于OAEP填充的RSA加密
// 使用SHA-256作为哈希函数
type RSAEncrypter struct {
	publicKey *rsa.PublicKey // RSA公钥
}

// NewRSAEncrypter 创建一个新的RSA加密器实例
// 参数: pubKey - PEM格式的公钥字节数组(PKIX格式)
// 返回: RSA加密器实例和可能的错误
func NewRSAEncrypter(pubKey []byte) (*RSAEncrypter, error) {
	// 解码PEM块
	block, _ := pem.Decode(pubKey)
	if block == nil {
		return nil, ErrInvalidPEMBlock
	}

	// 解析PKIX格式的公钥
	pubInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析PKIX公钥失败: %w", err)
	}

	// 类型断言为RSA公钥
	publicKey, ok := pubInterface.(*rsa.PublicKey)
	if !ok {
		return nil, ErrInvalidPublicKey
	}

	return &RSAEncrypter{publicKey: publicKey}, nil
}

// Encrypt 使用RSA-OAEP和SHA-256哈希函数加密数据
// 参数: originData - 待加密的明文
// 返回: 加密后的密文和可能的错误
func (e *RSAEncrypter) Encrypt(originData []byte) ([]byte, error) {
	if len(originData) == 0 {
		return nil, errors.New("原始数据为空")
	}

	// 使用SHA-256哈希函数进行OAEP填充
	hash := sha256.New()
	encryptedData, err := rsa.EncryptOAEP(hash, rand.Reader, e.publicKey, originData, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA加密失败: %w", err)
	}
	return encryptedData, nil
}

// ==================== AES-GCM 加密/解密 ====================

// AESGCMCrypter 实现使用AES-GCM模式的认证加密
// 这是推荐的方法,因为它同时提供机密性和真实性保护
// GCM模式会自动生成认证标签,防止密文被篡改
type AESGCMCrypter struct {
	aead cipher.AEAD // 认证加密接口
}

// NewAESGCMCrypter 创建一个新的AES-GCM加密器实例
// 参数: key - AES密钥,长度必须是16(AES-128)、24(AES-192)或32(AES-256)字节
// 返回: AES-GCM加密器实例和可能的错误
func NewAESGCMCrypter(key []byte) (*AESGCMCrypter, error) {
	keyLen := len(key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, ErrInvalidAESKeyLength
	}

	// 创建AES密码块
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("创建AES密码块失败: %w", err)
	}

	// 创建GCM模式封装器
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM失败: %w", err)
	}

	return &AESGCMCrypter{aead: aead}, nil
}

// Encrypt 使用AES-GCM加密明文
// 会自动生成随机nonce并附加到密文前面
// 参数: plaintext - 待加密的明文
// 返回: nonce+密文+认证标签 的组合字节数组和可能的错误
func (c *AESGCMCrypter) Encrypt(plaintext []byte) ([]byte, error) {
	// 生成随机nonce(number used once)
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成nonce失败: %w", err)
	}

	// Seal将密文和认证标签附加到nonce后面
	ciphertext := c.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt 使用AES-GCM解密密文并验证认证标签
// 参数: ciphertext - 包含nonce+密文+认证标签的字节数组
// 返回: 解密后的明文和可能的错误
// 注意: 如果认证标签验证失败,会返回错误,表示数据可能被篡改
func (c *AESGCMCrypter) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	// 提取nonce和加密数据
	nonce, encrypted := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// 解密并验证认证标签
	plaintext, err := c.aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("AES-GCM解密失败: %w", err)
	}
	return plaintext, nil
}

// ==================== AES-CTR 模式加密/解密 ====================

// AESCTRCrypter 实现CTR(Counter,计数器)模式的AES加密
// 注意: CTR模式不提供认证功能。如需认证加密,请使用AES-GCM
// CTR模式将块密码转换为流密码,适合加密任意长度的数据
type AESCTRCrypter struct {
	key []byte // AES密钥
}

// NewAESCTRCrypter 创建一个新的AES-CTR加密器实例
// 参数: key - AES密钥,长度必须是16、24或32字节
// 返回: AES-CTR加密器实例和可能的错误
func NewAESCTRCrypter(key []byte) (*AESCTRCrypter, error) {
	keyLen := len(key)
	if keyLen != 16 && keyLen != 24 && keyLen != 32 {
		return nil, ErrInvalidAESKeyLength
	}

	return &AESCTRCrypter{key: key}, nil
}

// Encrypt 使用AES-CTR模式加密明文
// 会生成随机IV(初始化向量)并附加到密文前面
// 参数: plaintext - 待加密的明文
// 返回: IV+密文 的组合字节数组和可能的错误
func (c *AESCTRCrypter) Encrypt(plaintext []byte) ([]byte, error) {
	// 创建AES密码块
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("创建AES密码块失败: %w", err)
	}

	// 为IV和密文分配空间
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]

	// 生成随机IV
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("生成IV失败: %w", err)
	}

	// 创建CTR模式流并加密
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

// Decrypt 使用AES-CTR模式解密密文
// 参数: ciphertext - 包含IV+密文的字节数组
// 返回: 解密后的明文和可能的错误
func (c *AESCTRCrypter) Decrypt(ciphertext []byte) ([]byte, error) {
	// 验证密文最小长度(必须包含IV)
	if len(ciphertext) < aes.BlockSize {
		return nil, ErrInvalidCiphertext
	}

	// 创建AES密码块
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("创建AES密码块失败: %w", err)
	}

	// 提取IV和加密数据
	iv := ciphertext[:aes.BlockSize]
	encrypted := ciphertext[aes.BlockSize:]

	// 使用CTR模式解密
	plaintext := make([]byte, len(encrypted))
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, encrypted)

	return plaintext, nil
}
