package helpers

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// GenerateRandom4DigitNumber generates a random 4-digit number (1000-9999)
func GenerateRandom4DigitNumber() (int, error) {
	// Generate a random number between 0 and 8999
	n, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random number: %w", err)
	}

	// Add 1000 to get a number between 1000 and 9999
	return int(n.Int64()) + 1000, nil
}

// GenerateRandom4DigitString generates a random 4-digit number as a zero-padded string
func GenerateRandom4DigitString() (string, error) {
	num, err := GenerateRandom4DigitNumber()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%04d", num), nil
}
