package helpers

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
)

func GenerateToken() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Generate6DigitCode generates a secure 6-digit number code
func Generate6DigitCode() (string, error) {
	// Generate a random number between 100000 and 999999 (6 digits)
	min := big.NewInt(100000)
	max := big.NewInt(999999)

	// Calculate the range (max - min + 1)
	rangeSize := new(big.Int).Sub(max, min)
	rangeSize.Add(rangeSize, big.NewInt(1))

	// Generate random number in range
	randomNum, err := rand.Int(rand.Reader, rangeSize)
	if err != nil {
		return "", err
	}

	// Add minimum value to get final result
	result := new(big.Int).Add(randomNum, min)

	return result.String(), nil
}
