package xnet

import (
	"crypto/rand"
	"fmt"
	"log/slog"
	"net"

	"github.com/wildmap/utility"
)

// IConn defines the interface for network connections.
// It provides methods for reading/writing messages and accessing connection addresses.
type IConn interface {
	// ReadMsg reads a complete message from the connection.
	// Returns the message data or an error if reading fails.
	ReadMsg() ([]byte, error)

	// WriteMsg writes a complete message to the connection.
	// Returns an error if writing fails.
	WriteMsg(args []byte) error

	// LocalAddr returns the local network address.
	LocalAddr() net.Addr

	// RemoteAddr returns the remote network address.
	RemoteAddr() net.Addr

	// Close closes the connection.
	// Any blocked Read or Write operations will be unblocked and return errors.
	Close() error
}

// IAgent defines the interface for network agents that handle client connections.
// Implementations must provide initialization, main processing loop, and cleanup logic.
type IAgent interface {
	// OnInit performs initialization tasks when a connection is established.
	// Returns an error if initialization fails.
	OnInit() error

	// Run executes the main processing loop for handling client requests.
	// This method blocks until the connection is closed or an error occurs.
	Run()

	// OnClose performs cleanup tasks when the connection is being closed.
	OnClose()
}

// ====================================== Agent Example ======================================

// Private key used for RSA decryption during the handshake process
var priKey = []byte(`-----BEGIN PRIVATE KEY-----
MIIBVAIBADANBgkqhkiG9w0BAQEFAASCAT4wggE6AgEAAkEA2xtaQzcpty+8qYP2
F9fzRZGuX1Op7UnX/a/PP4xkbEHjiMA+Q4eJSEEatGVuaOIxrj5HrYKrgxqFgbWc
9Eu2yQIDAQABAkAN8CKAzhyIO7ArtGpOP/2Iumi2RbM0lhL4X1u2ti6ZOEWlWQuN
abyMYyzH85lf5wkcN6FJc3TyhhLz8UDTOhg1AiEA84PSf0zxb6DNKHO6TcuqQ3u7
Gc3hSgOTR/QAtI/LRZcCIQDmVyt297w8R34w0TGlpjhMUQwFdrQ1OdTjp5twUKwy
nwIhAJXkCmm5XtOrUx0XPxIrzv4C50QW6hm44atkkhqSeDi5AiB3NPPEnQ9o7uMK
5qjX/r8yF9ut1DINPcHEk9Bo/wcvJwIgN/r4e6kLIkNLQiJtp0Tan6hu16E7xzDc
qloxpHJUTiE=
-----END PRIVATE KEY-----`)

// Public key used for reference (not used in this implementation)
var pubKey = []byte(`-----BEGIN PUBLIC KEY-----
MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANsbWkM3KbcvvKmD9hfX80WRrl9Tqe1J
1/2vzz+MZGxB44jAPkOHiUhBGrRlbmjiMa4+R62Cq4MahYG1nPRLtskCAwEAAQ==
-----END PUBLIC KEY-----`)

// AgentExample is a sample implementation of the IAgent interface.
// It handles encrypted communication using RSA for key exchange and AES for session encryption.
type AgentExample struct {
	conn    IConn            // Network connection interface
	crypter utility.ICrypter // Encryption/decryption handler for session data
}

// NewAgent creates a new AgentExample instance with the given connection.
func NewAgent(conn IConn) *AgentExample {
	return &AgentExample{
		conn: conn,
	}
}

// OnInit performs the encryption handshake:
// 1. Receives RSA-encrypted temporary key from client
// 2. Decrypts temporary key using server's private key
// 3. Generates random session key
// 4. Encrypts session key with temporary key and sends to client
// 5. Sets up AES encrypter for subsequent communication
func (agent *AgentExample) OnInit() error {
	// Read the first message containing the encrypted temporary key
	data, err := agent.conn.ReadMsg()
	if err != nil {
		return err
	}

	// Decrypt the temporary key using server's RSA private key
	rsa, err := utility.NewRSADecrypter(priKey)
	if err != nil {
		return fmt.Errorf("%v new crypter err, err: %v", agent, err)
	}
	tempKey, err := rsa.Decrypt(data)
	if err != nil {
		return fmt.Errorf("%v crypter decrypt data err, err: %v, data: %v", agent, err, data)
	}
	slog.Info(fmt.Sprintf("%v tempKey len: %v data: %v", agent, len(tempKey), tempKey))

	// Generate a random session key for AES encryption
	sessionKey := make([]byte, 16)
	_, err = rand.Read(sessionKey)
	if err != nil {
		return fmt.Errorf("%v rand.Read err %v", agent, err)
	}
	slog.Info(fmt.Sprintf("%v sessionKey len: %v data: %v", agent, len(sessionKey), sessionKey))

	// Create AES encrypter with the session key for this agent
	agent.crypter, err = utility.NewAESCTRCrypter(sessionKey)
	if err != nil {
		return fmt.Errorf("%v sessionKey NewAESCrypter err: %v", agent, err)
	}

	// Encrypt the session key using the temporary key
	crypter, err := utility.NewAESCTRCrypter(tempKey)
	if err != nil {
		return fmt.Errorf("%v tempKey NewAESCrypter err: %v", agent, err)
	}
	cryptedData, err := crypter.Encrypt(sessionKey)
	if err != nil {
		return fmt.Errorf("%v crypter.Encrypt err: %v", agent, err)
	}

	// Send encrypted session key back to client
	return agent.conn.WriteMsg(cryptedData)
}

// readMessage reads and decrypts a single message from the connection.
// Returns the decrypted message data or an error.
func (agent *AgentExample) readMessage() ([]byte, error) {
	data, err := agent.conn.ReadMsg()
	if err != nil {
		return nil, err
	}
	originData, err := agent.crypter.Decrypt(data)
	if err != nil {
		return nil, err
	}
	return originData, nil
}

// writeMessage encrypts and writes a single message to the connection.
// Returns an error if encryption or writing fails.
func (agent *AgentExample) writeMessage(data []byte) error {
	cryptedData, err := agent.crypter.Encrypt(data)
	if err != nil {
		return err
	}
	return agent.conn.WriteMsg(cryptedData)
}

// Run continuously reads and processes messages from the client.
// This method blocks until an error occurs or the connection is closed.
func (agent *AgentExample) Run() {
	for {
		data, err := agent.readMessage()
		if err != nil {
			slog.Error(fmt.Sprintf("AgentExample ReadMsg err:%v", err))
			break
		}
		fmt.Println(string(data))
	}
}

// OnClose cleans up resources by closing the underlying connection.
func (agent *AgentExample) OnClose() {
	_ = agent.conn.Close()
}
