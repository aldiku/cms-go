package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

type CryptoEngine struct {
	key []byte
}

// NewCryptoEngine creates a new encryption engine with the given passphrase
func NewCryptoEngine(passphrase string) *CryptoEngine {
	hash := sha256.Sum256([]byte(passphrase))
	return &CryptoEngine{key: hash[:]}
}

// Encrypt transforms a string into encrypted text
func (c *CryptoEngine) Encrypt(plaintext string) (string, error) {
	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}

	// Create AES cipher
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	// Encrypt the plaintext
	plaintextBytes := []byte(plaintext)
	ciphertext := make([]byte, len(plaintextBytes))
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext, plaintextBytes)

	// Combine IV + ciphertext and encode as base64
	encryptedData := append(iv, ciphertext...)
	encryptedBase64 := base64.StdEncoding.EncodeToString(encryptedData)

	// Apply reversible transformation to make it look random
	return c.reversibleTransform(encryptedBase64, true), nil
}

// Decrypt reverses the encryption process
func (c *CryptoEngine) Decrypt(encryptedText string) (string, error) {
	// Reverse the transformation
	encryptedBase64 := c.reversibleTransform(encryptedText, false)

	// Decode base64
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", err
	}

	if len(encryptedData) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	// Extract IV and ciphertext
	iv := encryptedData[:aes.BlockSize]
	ciphertext := encryptedData[aes.BlockSize:]

	// Create AES cipher
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}

	// Decrypt
	stream := cipher.NewCFBDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return string(plaintext), nil
}

// reversibleTransform applies a reversible transformation to make base64 look random
func (c *CryptoEngine) reversibleTransform(input string, encrypt bool) string {
	if !encrypt {
		// Decryption: reverse the process
		return c.decodeCustomBase64(input)
	}

	// Encryption: convert to custom base64 with random-looking characters
	return c.encodeToCustomBase64(input)
}

// encodeToCustomBase64 converts standard base64 to a custom character set
func (c *CryptoEngine) encodeToCustomBase64(input string) string {
	// Custom character sets that look random
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numbers := "0123456789"
	special := "!@#$%^&*"
	allChars := letters + numbers + special

	// Create a mapping from standard base64 to custom characters
	standardBase64 := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="

	var result strings.Builder

	for _, char := range input {
		if idx := strings.IndexRune(standardBase64, char); idx != -1 {
			// Map to custom character set
			result.WriteByte(allChars[idx%len(allChars)])
		} else {
			// Keep original if not in base64 (shouldn't happen)
			result.WriteRune(char)
		}
	}

	return result.String()
}

// decodeCustomBase64 reverses the custom base64 encoding
func (c *CryptoEngine) decodeCustomBase64(input string) string {
	// Custom character sets
	letters := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numbers := "0123456789"
	special := "!@#$%^&*"
	allChars := letters + numbers + special

	// Standard base64 characters
	standardBase64 := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="

	var result strings.Builder

	for _, char := range input {
		if idx := strings.IndexRune(allChars, char); idx != -1 {
			// Map back to standard base64
			result.WriteByte(standardBase64[idx%len(standardBase64)])
		} else {
			// Keep original if not in custom set
			result.WriteRune(char)
		}
	}

	return result.String()
}

// SimpleEncrypt is a convenience function for one-time encryption
func SimpleEncrypt(plaintext, passphrase string) (string, error) {
	engine := NewCryptoEngine(passphrase)
	return engine.Encrypt(plaintext)
}

// SimpleDecrypt is a convenience function for one-time decryption
func SimpleDecrypt(encryptedText, passphrase string) (string, error) {
	engine := NewCryptoEngine(passphrase)
	return engine.Decrypt(encryptedText)
}

// Alternative: Number-rich encryption
type NumberCryptoEngine struct {
	key []byte
}

func NewNumberCryptoEngine(passphrase string) *NumberCryptoEngine {
	hash := sha256.Sum256([]byte(passphrase))
	return &NumberCryptoEngine{key: hash[:]}
}

func (n *NumberCryptoEngine) Encrypt(plaintext string) (string, error) {
	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}

	block, err := aes.NewCipher(n.key)
	if err != nil {
		return "", err
	}

	// Encrypt
	plaintextBytes := []byte(plaintext)
	ciphertext := make([]byte, len(plaintextBytes))
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext, plaintextBytes)

	// Combine IV + ciphertext
	encryptedData := append(iv, ciphertext...)

	// Convert to hex (already looks like random numbers/letters)
	hexStr := hex.EncodeToString(encryptedData)

	// Add some formatting to make it look more like random data
	return n.formatAsRandomData(hexStr), nil
}

func (n *NumberCryptoEngine) Decrypt(encryptedText string) (string, error) {
	// Remove formatting
	cleanHex := n.removeFormatting(encryptedText)

	// Decode hex
	encryptedData, err := hex.DecodeString(cleanHex)
	if err != nil {
		return "", err
	}

	if len(encryptedData) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	// Extract IV and ciphertext
	iv := encryptedData[:aes.BlockSize]
	ciphertext := encryptedData[aes.BlockSize:]

	block, err := aes.NewCipher(n.key)
	if err != nil {
		return "", err
	}

	// Decrypt
	stream := cipher.NewCFBDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return string(plaintext), nil
}

func (n *NumberCryptoEngine) formatAsRandomData(hexStr string) string {
	var result strings.Builder
	groupSize := 4
	count := 0

	for i, char := range hexStr {
		result.WriteRune(char)
		count++

		// Add spaces and numbers periodically to make it look random
		if count == groupSize && i < len(hexStr)-1 {
			// Add a random-looking separator
			separators := []string{"-", "_", ".", ":", ""}
			separator := separators[i%len(separators)]
			if separator != "" {
				result.WriteString(separator)
			}
			count = 0
			groupSize = 3 + (i % 4) // Vary group size
		}
	}

	return result.String()
}

func (n *NumberCryptoEngine) removeFormatting(input string) string {
	var result strings.Builder

	for _, char := range input {
		// Keep only hex characters (0-9, a-f, A-F)
		if (char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'f') ||
			(char >= 'A' && char <= 'F') {
			result.WriteRune(char)
		}
	}

	return result.String()
}

func TestEncript() {
	// Example usage
	passphrase := os.Getenv("APP_KEY")
	// plaintext := "12345&23562653265&s"

	// fmt.Println("=== Fixed Basic Encryption ===")
	// fmt.Printf("Original: %s\n", plaintext)

	// // Basic encryption
	// encrypted, err := SimpleEncrypt(plaintext, passphrase)
	// if err != nil {
	// 	fmt.Printf("Encryption error: %v\n", err)
	// 	return
	// }
	// fmt.Printf("Encrypted: %s\n", encrypted)

	// decrypted, err := SimpleDecrypt(encrypted, passphrase)
	// if err != nil {
	// 	fmt.Printf("Decryption error: %v\n", err)
	// 	return
	// }
	// fmt.Printf("Decrypted: %s\n", decrypted)
	// fmt.Printf("Match: %t\n\n", plaintext == decrypted)

	// fmt.Println("=== Number-rich Encryption ===")
	// // Number-rich encryption
	// numberEngine := NewNumberCryptoEngine(passphrase)
	// numberEncrypted, err := numberEngine.Encrypt(plaintext)
	// if err != nil {
	// 	fmt.Printf("Number encryption error: %v\n", err)
	// 	return
	// }
	// fmt.Printf("Number Encrypted: %s\n", numberEncrypted)

	// numberDecrypted, err := numberEngine.Decrypt(numberEncrypted)
	// if err != nil {
	// 	fmt.Printf("Number decryption error: %v\n", err)
	// 	return
	// }
	// fmt.Printf("Number Decrypted: %s\n", numberDecrypted)
	// fmt.Printf("Match: %t\n\n", plaintext == numberDecrypted)

	// Test with various messages
	fmt.Println("=== Testing Various Messages ===")
	testMessages := []string{
		"12345&23562653265&s",
		"12345&23562653265&p",
	}

	for i, msg := range testMessages {
		fmt.Printf("\nTest %d:\n", i+1)
		fmt.Printf("  Original:  %s\n", msg)

		enc, err := SimpleEncrypt(msg, passphrase)
		if err != nil {
			fmt.Printf("  Encryption failed: %v\n", err)
			continue
		}
		fmt.Printf("  Encrypted: %s\n", enc)

		dec, err := SimpleDecrypt(enc, passphrase)
		if err != nil {
			fmt.Printf("  Decryption failed: %v\n", err)
			continue
		}
		fmt.Printf("  Decrypted: %s\n", dec)
		fmt.Printf("  Success: %t\n", msg == dec)
	}
}
