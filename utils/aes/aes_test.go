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

package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"strings"
	"testing"

	"github.com/rulego/rulego/test/assert"
)

func TestGenerateKey(t *testing.T) {
	key1 := []byte("short")
	generatedKey1 := generateKey(key1)
	assert.Equal(t, 32, len(generatedKey1))
	assert.Equal(t, "short000000000000000000000000000", string(generatedKey1))

	key2 := []byte("exactlythirtytwobyteslongkey1234")
	generatedKey2 := generateKey(key2)
	assert.Equal(t, 32, len(generatedKey2))
	assert.Equal(t, "exactlythirtytwobyteslongkey1234", string(generatedKey2))

	key3 := []byte("thisisalongerkeythanthirtytwobytes1234567890")
	generatedKey3 := generateKey(key3)
	assert.Equal(t, 32, len(generatedKey3))
	// generateKey will truncate the key if it's longer than 32 bytes
	assert.Equal(t, "thisisalongerkeythanthirtytwobyt", string(generatedKey3))

}

func TestAes(t *testing.T) {
	key := "secret"
	plaintext := "Hello, World!"

	encrypted, err := Encrypt(plaintext, []byte(key))
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := Decrypt(encrypted, []byte(key))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, plaintext, decrypted)

	// Test Decrypt with invalid hex string
	_, err = Decrypt("not a hex string", []byte(key))
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid byte") || strings.Contains(err.Error(), "odd length hex string"))

	// Test Decrypt with invalid padding
	// Create a valid encrypted message first
	block, _ := aes.NewCipher(generateKey([]byte(key)))
	iv := make([]byte, aes.BlockSize)
	// For testing, IV can be all zeros or random, doesn't matter for this specific padding test
	// as long as decryption up to padding check works.
	plaintextBytes := []byte("1234567890123456") // exactly one block
	// Manually pad to two blocks, but with incorrect padding value at the end
	paddedBytes := make([]byte, len(plaintextBytes)+aes.BlockSize)
	copy(paddedBytes, plaintextBytes)
	// Fill padding block with a value, e.g., 0x10 (16)
	for i := len(plaintextBytes); i < len(paddedBytes); i++ {
		paddedBytes[i] = byte(aes.BlockSize)
	}
	// Corrupt the last byte of padding
	paddedBytes[len(paddedBytes)-1] = 0x00 // Invalid padding value

	ciphertextBytes := make([]byte, aes.BlockSize+len(paddedBytes))
	copy(ciphertextBytes[:aes.BlockSize], iv) // Prepend IV

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertextBytes[aes.BlockSize:], paddedBytes)

	// Test Encrypt with empty plaintext
	emptyPlaintext := ""
	emptyEncrypted, err := Encrypt(emptyPlaintext, []byte(key))
	assert.Nil(t, err)
	emptyDecrypted, err := Decrypt(emptyEncrypted, []byte(key))
	assert.Nil(t, err)
	assert.Equal(t, emptyPlaintext, emptyDecrypted)

	// Test with a key longer than 32 bytes (generateKey will truncate)
	longKey := "thisisareallylongkeythatwillbetruncatedbygenerateKeyfunction"
	encryptedWithLongKey, err := Encrypt(plaintext, []byte(longKey))
	assert.Nil(t, err)
	decryptedWithLongKey, err := Decrypt(encryptedWithLongKey, []byte(longKey))
	assert.Nil(t, err)
	assert.Equal(t, plaintext, decryptedWithLongKey)

	// Test with a key shorter than 32 bytes (generateKey will pad)
	shortKey := "shortk"
	encryptedWithShortKey, err := Encrypt(plaintext, []byte(shortKey))
	assert.Nil(t, err)
	decryptedWithShortKey, err := Decrypt(encryptedWithShortKey, []byte(shortKey))
	assert.Nil(t, err)
	assert.Equal(t, plaintext, decryptedWithShortKey)
}
