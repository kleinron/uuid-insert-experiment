// Package uuidx generates UUIDv4 and UUIDv7 as raw 16-byte values for BINARY(16) PKs.
// This package is used only on the measure path (EXPERIMENT_ARM). Preload/season
// must not call it — those use the shared key source in package datagen.
package uuidx

import (
	"fmt"

	"github.com/google/uuid"
)

// V4 returns a random RFC 9562 UUIDv4 as 16 bytes.
func V4() [16]byte {
	return [16]byte(uuid.New())
}

// V7 returns a time-ordered RFC 9562 UUIDv7 as 16 bytes.
func V7() ([16]byte, error) {
	u, err := uuid.NewV7()
	if err != nil {
		return [16]byte{}, fmt.Errorf("uuidv7: %w", err)
	}
	return [16]byte(u), nil
}

// ForArm returns a 16-byte UUID for EXPERIMENT_ARM v4 or v7.
func ForArm(arm string) ([16]byte, error) {
	switch arm {
	case "v4":
		return V4(), nil
	case "v7":
		return V7()
	default:
		return [16]byte{}, fmt.Errorf("unknown experiment arm %q (want v4 or v7)", arm)
	}
}

// Version returns the RFC UUID version nibble (4 or 7 for this package).
func Version(id [16]byte) int {
	return int(id[6] >> 4)
}
