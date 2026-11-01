package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const passwordMemoryKiB = 64 * 1024
const passwordIterations = 3
const passwordParallelism = 2
const passwordKeyLength = 32

func NormalizeEmail(input string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(input))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) < 3 || len(email) > 254 {
		return "", errors.New("enter a valid email address")
	}
	return email, nil
}

func HashPassword(password string) (string, error) {
	if len(password) < 12 || len(password) > 1024 {
		return "", errors.New("password must contain 12–1024 bytes")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, passwordIterations, passwordMemoryKiB, passwordParallelism, passwordKeyLength)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", passwordMemoryKiB, passwordIterations, passwordParallelism, enc.EncodeToString(salt), enc.EncodeToString(hash)), nil
}

func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" {
		return false
	}
	var memory, iterations, parallel int
	for i, part := range strings.Split(parts[2], ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return false
		}
		n, err := strconv.Atoi(kv[1])
		if err != nil {
			return false
		}
		switch i {
		case 0:
			if kv[0] != "m" {
				return false
			}
			memory = n
		case 1:
			if kv[0] != "t" {
				return false
			}
			iterations = n
		case 2:
			if kv[0] != "p" {
				return false
			}
			parallel = n
		default:
			return false
		}
	}
	if memory != passwordMemoryKiB || iterations != passwordIterations || parallel != passwordParallelism {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[3])
	if err != nil || len(salt) != 16 {
		return false
	}
	want, err := enc.DecodeString(parts[4])
	if err != nil || len(want) != passwordKeyLength {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(memory), uint8(parallel), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
