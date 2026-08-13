package filestore

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strconv"
	"strings"
)

// 散列格式：pbkdf2$sha256$<iter>$<saltHex>$<dkHex>
// 选 pbkdf2：Go 1.24 进了标准库，无需引依赖。
const (
	pwHashPrefix = "pbkdf2$sha256$"
	pwIterations = 210000
	pwSaltLen    = 16
	pwKeyLen     = 32
)

// IsHashedPassword 判断是否为本模块产出的散列值。
func IsHashedPassword(s string) bool {
	return strings.HasPrefix(s, pwHashPrefix)
}

// HashPassword 生成加盐散列。空密码原样返回（调用方用空值表示「不改密码」）。
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", nil
	}
	salt := make([]byte, pwSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk, err := pbkdf2.Key(sha256.New, password, salt, pwIterations, pwKeyLen)
	if err != nil {
		return "", err
	}
	return pwHashPrefix + strconv.Itoa(pwIterations) + "$" +
		hex.EncodeToString(salt) + "$" + hex.EncodeToString(dk), nil
}

// VerifyPassword 校验密码。stored 非散列格式时按明文比较，
// 以兼容历史明文行与 config.conf 内置账号。
func VerifyPassword(stored, password string) bool {
	if stored == "" {
		return false
	}
	if !IsHashedPassword(stored) {
		return subtle.ConstantTimeCompare([]byte(stored), []byte(password)) == 1
	}
	parts := strings.Split(strings.TrimPrefix(stored, pwHashPrefix), "$")
	if len(parts) != 3 {
		return false
	}
	iter, err := strconv.Atoi(parts[0])
	if err != nil || iter <= 0 {
		return false
	}
	salt, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(parts[2])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}
