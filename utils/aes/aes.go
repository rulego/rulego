/*
 * Copyright 2024 The RuleGo Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package aes provides encryption and decryption functions using the AES algorithm.
// It includes functions for generating AES keys, encrypting plaintext, and decrypting ciphertext.
//
// The package uses AES-256 encryption in CBC mode with PKCS7 padding.
// Key generation pads or truncates the provided key to ensure it's always 32 bytes (256 bits).
//
// Usage:
//
//	key := []byte("your-secret-key")
//	plaintext := "Hello, World!"
//
//	// Encrypt
//	encrypted, err := aes.Encrypt(plaintext, key)
//	if err != nil {
//	    // Handle error
//	}
//
//	// Decrypt
//	decrypted, err := aes.Decrypt(encrypted, key)
//	if err != nil {
//	    // Handle error
//	}
//
// Note: Always use a secure method to generate and store your encryption keys.
// Never use hardcoded keys in production environments.
package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"io"
)

// generateKey 根据给定的字符串生成一个AES密钥
func generateKey(key []byte) []byte {
	newKey := make([]byte, 32) // AES-256
	copy(newKey, key)
	for i := len(key); i < 32; i++ {
		newKey[i] = '0'
	}
	return newKey
}

// Encrypt 使用AES-256加密数据
func Encrypt(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(generateKey(key))
	if err != nil {
		return "", err
	}

	// 原始数据填充
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	plaintext += string(padtext)

	// 加密
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext[aes.BlockSize:], []byte(plaintext))

	return hex.EncodeToString(ciphertext), nil
}

// Decrypt 使用AES-256解密数据
func Decrypt(encrypted string, key []byte) (string, error) {
	block, err := aes.NewCipher(generateKey(key))
	if err != nil {
		return "", err
	}

	ciphertext, err := hex.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < aes.BlockSize {
		return "", err
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// 移除填充
	padding := int(ciphertext[len(ciphertext)-1])
	if padding < 1 || padding > aes.BlockSize {
		return "", err
	}
	for i := len(ciphertext) - padding; i < len(ciphertext); i++ {
		if ciphertext[i] != byte(padding) {
			return "", err
		}
	}

	return string(ciphertext[:len(ciphertext)-padding]), nil
}
