// Package utils provides internal utility functions and types.
package utils

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
)

// SecureIntn returns a secure random integer in the range [0, n).
// It uses crypto/rand instead of math/rand for better security.
func SecureIntn(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("invalid input: n must be positive")
	}

	// For small ranges, use Int which is more efficient
	if n <= 1<<31-1 {
		max := big.NewInt(int64(n))
		val, err := rand.Int(rand.Reader, max)
		if err != nil {
			return 0, err
		}
		return int(val.Int64()), nil
	}

	// For larger ranges, use this method
	// Calculate how many bytes we need to read
	numBytes := 4 // Using 4 bytes (32 bits) for integers
	b := make([]byte, numBytes)

	// Read random bytes
	_, err := rand.Read(b)
	if err != nil {
		return 0, err
	}

	// Convert bytes to uint32
	val := binary.BigEndian.Uint32(b)

	// Map to the range we want
	result := int(val) % n

	return result, nil
}

// MustSecureIntn is like SecureIntn but panics on error.
// This is useful for places where errors are not expected and would indicate a serious problem.
func MustSecureIntn(n int) int {
	result, err := SecureIntn(n)
	if err != nil {
		panic(fmt.Sprintf("failed to generate random number: %v", err))
	}
	return result
}
