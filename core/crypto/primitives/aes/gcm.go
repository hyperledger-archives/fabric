package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"github.com/hyperledger/fabric/core/crypto/primitives"
	"github.com/hyperledger/fabric/core/crypto/utils"
	"io"
)

type aesSecretKeyImpl struct {
	key []byte
}

func (sk *aesSecretKeyImpl) GetRand() io.Reader {
	return nil
}

type aes256GSMStreamCipherImpl struct {
	forEncryption bool
	gcm           cipher.AEAD
	nonceSize     int
}

// Init initializes this cipher with the passed parameters
func (sc *aes256GSMStreamCipherImpl) Init(forEncryption bool, params primitives.CipherParameters) error {
	var aesKey *aesSecretKeyImpl

	switch sk := params.(type) {
	case *aesSecretKeyImpl:
		if len(sk.key) != 32 {
			return fmt.Errorf("Invalid key lentgh. Len was [%d], expected [32].", len(sk.key))
		}
		aesKey = sk
	default:
		return primitives.ErrInvalidSecretKeyType
	}

	// Init aes
	c, err := aes.NewCipher(aesKey.key)
	if err != nil {
		return err
	}

	// Init gcm for encryption
	sc.gcm, err = cipher.NewGCM(c)
	if err != nil {
		return err
	}

	sc.forEncryption = forEncryption
	sc.nonceSize = sc.gcm.NonceSize()

	return nil
}

// Process processes the byte array given in input
func (sc *aes256GSMStreamCipherImpl) Process(msg []byte) ([]byte, error) {
	if sc.forEncryption {
		nonce, err := sc.generateNonce()
		if err != nil {
			return nil, primitives.ErrEncryption
		}

		// Seal will append the output to the first argument; the usage
		// here appends the ciphertext to the nonce. The final parameter
		// is any additional data to be authenticated.
		out := sc.gcm.Seal(nonce, nonce, msg, nil)
		return out, nil
	}

	if len(msg) <= sc.nonceSize {
		return nil, primitives.ErrDecryption
	}

	nonce := make([]byte, sc.nonceSize)
	copy(nonce, msg[:])

	// Decrypt the message, using the sender ID as the additional
	// data requiring authentication.
	out, err := sc.gcm.Open(nil, nonce, msg[sc.nonceSize:], nil)
	if err != nil {
		return nil, primitives.ErrDecryption
	}
	return out, nil
}

func (sc *aes256GSMStreamCipherImpl) generateNonce() ([]byte, error) {
	return primitives.GetRandomBytes(sc.nonceSize)
}

type aes256GSMStreamCipherSPIImpl struct {
}

func (spi *aes256GSMStreamCipherSPIImpl) GenerateKey() (primitives.SecretKey, error) {
	key, err := primitives.GetRandomBytes(32)
	if err != nil {
		return nil, err
	}

	return &aesSecretKeyImpl{key}, nil
}

func (spi *aes256GSMStreamCipherSPIImpl) GenerateKeyAndSerialize() (primitives.SecretKey, []byte, error) {
	key, err := primitives.GetRandomBytes(32)
	if err != nil {
		return nil, nil, err
	}

	return &aesSecretKeyImpl{key}, utils.Clone(key), nil
}

func (spi *aes256GSMStreamCipherSPIImpl) NewSecretKey(rand io.Reader, params interface{}) (primitives.SecretKey, error) {
	switch t := params.(type) {
	case []byte:
		if len(t) != 32 {
			return nil, fmt.Errorf("Invalid key lentgh. Len was [%d], expected [32].", len(t))
		}
		return &aesSecretKeyImpl{t}, nil
	default:
		return nil, primitives.ErrInvalidKeyGeneratorParameter
	}
}

// NewStreamCipherForEncryptionFromKey creates a new StreamCipher for encryption from a secret key
func (spi *aes256GSMStreamCipherSPIImpl) NewStreamCipherForEncryptionFromKey(secret primitives.SecretKey) (primitives.StreamCipher, error) {
	sc := aes256GSMStreamCipherImpl{}
	if err := sc.Init(true, secret); err != nil {
		return nil, err
	}

	return &sc, nil
}

// NewStreamCipherForDecryptionFromKey creates a new StreamCipher for decryption from a secret key
func (spi *aes256GSMStreamCipherSPIImpl) NewStreamCipherForEncryptionFromSerializedKey(secret []byte) (primitives.StreamCipher, error) {
	key, err := spi.NewSecretKey(nil, secret)
	if err != nil {
		return nil, err
	}

	sc := aes256GSMStreamCipherImpl{}
	if err := sc.Init(true, key); err != nil {
		return nil, err
	}

	return &sc, nil
}

// NewStreamCipherForDecryptionFromKey creates a new StreamCipher for decryption from a secret key
func (spi *aes256GSMStreamCipherSPIImpl) NewStreamCipherForDecryptionFromKey(secret primitives.SecretKey) (primitives.StreamCipher, error) {
	sc := aes256GSMStreamCipherImpl{}
	if err := sc.Init(false, secret); err != nil {
		return nil, err
	}

	return &sc, nil
}

// NewStreamCipherForDecryptionFromKey creates a new StreamCipher for decryption from a secret key
func (spi *aes256GSMStreamCipherSPIImpl) NewStreamCipherForDecryptionFromSerializedKey(secret []byte) (primitives.StreamCipher, error) {
	key, err := spi.NewSecretKey(nil, secret)
	if err != nil {
		return nil, err
	}

	sc := aes256GSMStreamCipherImpl{}
	if err := sc.Init(false, key); err != nil {
		return nil, err
	}

	return &sc, nil
}

// SerializePrivateKey serializes a private key
func (spi *aes256GSMStreamCipherSPIImpl) SerializeSecretKey(secret primitives.SecretKey) ([]byte, error) {
	if secret == nil {
		return nil, nil
	}

	switch sk := secret.(type) {
	case *aesSecretKeyImpl:
		return utils.Clone(sk.key), nil
	default:
		return nil, primitives.ErrInvalidSecretKeyType
	}
}

// DeserializePrivateKey deserializes to a private key
func (spi *aes256GSMStreamCipherSPIImpl) DeserializeSecretKey(bytes []byte) (primitives.SecretKey, error) {
	if len(bytes) >= 32 {
		return &aesSecretKeyImpl{bytes[:32]}, nil
	}
	return nil, primitives.ErrInvalidKeyParameter
}

// NewAES256GSMSPI returns a new SPI instance for AES256 in GSM mode
func NewAES256GSMSPI() primitives.StreamCipherSPI {
	return &aes256GSMStreamCipherSPIImpl{}
}
