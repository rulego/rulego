/*
 * Copyright 2023 The RuleGo Authors.
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

package external

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test/assert"
)

// testKeyPair 生成测试用 RSA 私钥，返回明文 PEM 与加密 PEM（passphrase 为空时不生成加密版）。
// testKeyPair generates a test RSA key pair and returns the plain PEM and the passphrase-encrypted PEM.
func testKeyPair(t *testing.T, passphrase string) (plainPEM string, encryptedPEM string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.Nil(t, err)
	der := x509.MarshalPKCS1PrivateKey(key)
	plainPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
	if passphrase != "" {
		//nolint:staticcheck // EncryptPEMBlock is deprecated but still required to build encrypted PKCS#1 PEM in tests
		block, err := x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", der, []byte(passphrase), x509.PEMCipherAES256)
		assert.Nil(t, err)
		encryptedPEM = string(pem.EncodeToMemory(block))
	}
	return plainPEM, encryptedPEM
}

// TestSshNodeParseSigner 覆盖私钥解析的各类场景。
// TestSshNodeParseSigner covers private key parsing scenarios.
func TestSshNodeParseSigner(t *testing.T) {
	const passphrase = "test-passphrase"
	plainPEM, encryptedPEM := testKeyPair(t, passphrase)

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "id_rsa")
	assert.Nil(t, os.WriteFile(keyPath, []byte(plainPEM), 0600))

	tests := []struct {
		name        string
		config      SshConfiguration
		wantNil     bool
		wantErr     bool
		errContains string
	}{
		{"no private key", SshConfiguration{}, true, false, ""},
		{"plain PEM content", SshConfiguration{PrivateKey: plainPEM}, false, false, ""},
		{"plain PEM file", SshConfiguration{PrivateKeyPath: keyPath}, false, false, ""},
		{"privateKey content takes precedence over path", SshConfiguration{PrivateKey: plainPEM, PrivateKeyPath: filepath.Join(dir, "missing")}, false, false, ""},
		{"encrypted with correct passphrase", SshConfiguration{PrivateKey: encryptedPEM, PrivateKeyPassphrase: passphrase}, false, false, ""},
		{"encrypted with wrong passphrase", SshConfiguration{PrivateKey: encryptedPEM, PrivateKeyPassphrase: "wrong-passphrase"}, false, true, ""},
		{"encrypted without passphrase", SshConfiguration{PrivateKey: encryptedPEM}, false, true, "privateKeyPassphrase"},
		{"missing key file", SshConfiguration{PrivateKeyPath: filepath.Join(dir, "no-such-file")}, false, true, "read private key file"},
		{"invalid pem content", SshConfiguration{PrivateKey: "not a private key"}, false, true, "parse private key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &SshNode{Config: tt.config}
			signer, err := node.parseSigner()
			if tt.wantErr {
				assert.NotNil(t, err)
				if tt.errContains != "" {
					assert.True(t, strings.Contains(err.Error(), tt.errContains),
						"error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			assert.Nil(t, err)
			if tt.wantNil {
				assert.Nil(t, signer)
			} else {
				assert.NotNil(t, signer)
			}
		})
	}
}

// TestSshNodeInitPrivateKeyValidation 覆盖 Init 阶段私钥相关的校验。
// TestSshNodeInitPrivateKeyValidation covers private key validation in Init.
func TestSshNodeInitPrivateKeyValidation(t *testing.T) {
	plainPEM, _ := testKeyPair(t, "")

	t.Run("private key only passes validation", func(t *testing.T) {
		node := &SshNode{}
		err := node.Init(types.NewConfig(), types.Configuration{
			"host":       "127.0.0.1",
			"port":       22,
			"username":   "root",
			"privateKey": plainPEM,
			"cmd":        "ls",
		})
		assert.Nil(t, err)
		assert.NotNil(t, node.signer)
		node.Destroy()
	})

	t.Run("private key file only passes validation", func(t *testing.T) {
		dir := t.TempDir()
		keyPath := filepath.Join(dir, "id_rsa")
		assert.Nil(t, os.WriteFile(keyPath, []byte(plainPEM), 0600))
		node := &SshNode{}
		err := node.Init(types.NewConfig(), types.Configuration{
			"host":           "127.0.0.1",
			"port":           22,
			"username":       "root",
			"privateKeyPath": keyPath,
			"cmd":            "ls",
		})
		assert.Nil(t, err)
		assert.NotNil(t, node.signer)
		node.Destroy()
	})

	t.Run("passphrase without private key", func(t *testing.T) {
		node := &SshNode{}
		err := node.Init(types.NewConfig(), types.Configuration{
			"host":                 "127.0.0.1",
			"port":                 22,
			"username":             "root",
			"password":             "secret",
			"privateKeyPassphrase": "pass",
			"cmd":                  "ls",
		})
		assert.NotNil(t, err)
		assert.Equal(t, SshConfigPassphraseNoKeyErr.Error(), err.Error())
	})

	t.Run("neither password nor private key", func(t *testing.T) {
		node := &SshNode{}
		err := node.Init(types.NewConfig(), types.Configuration{
			"host":     "127.0.0.1",
			"port":     22,
			"username": "root",
			"cmd":      "ls",
		})
		assert.NotNil(t, err)
		assert.Equal(t, SshConfigEmptyErr.Error(), err.Error())
	})

	t.Run("invalid private key fails fast", func(t *testing.T) {
		node := &SshNode{}
		err := node.Init(types.NewConfig(), types.Configuration{
			"host":       "127.0.0.1",
			"port":       22,
			"username":   "root",
			"privateKey": "garbage",
			"cmd":        "ls",
		})
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "parse private key"))
	})
}

// TestSshNodeClientConfigAuth 覆盖 clientConfig 的认证方式组装。
// TestSshNodeClientConfigAuth covers auth method assembly in clientConfig.
func TestSshNodeClientConfigAuth(t *testing.T) {
	plainPEM, _ := testKeyPair(t, "")

	t.Run("password only", func(t *testing.T) {
		node := &SshNode{Config: SshConfiguration{Username: "root", Password: "secret"}}
		cfg := node.clientConfig()
		assert.Equal(t, 1, len(cfg.Auth))
		assert.True(t, strings.Contains(reflect.TypeOf(cfg.Auth[0]).String(), "password"),
			"auth method should be password type, got %s", reflect.TypeOf(cfg.Auth[0]))
	})

	t.Run("private key only", func(t *testing.T) {
		node := &SshNode{Config: SshConfiguration{Username: "root", PrivateKey: plainPEM}}
		signer, err := node.parseSigner()
		assert.Nil(t, err)
		node.signer = signer
		cfg := node.clientConfig()
		assert.Equal(t, 1, len(cfg.Auth))
		// 无私钥时不放空密码认证 - no empty password auth when password is empty
		assert.True(t, strings.Contains(reflect.TypeOf(cfg.Auth[0]).String(), "publicKey"),
			"auth method should be publicKey type, got %s", reflect.TypeOf(cfg.Auth[0]))
	})

	t.Run("password and private key coexist", func(t *testing.T) {
		node := &SshNode{Config: SshConfiguration{Username: "root", Password: "secret", PrivateKey: plainPEM}}
		signer, err := node.parseSigner()
		assert.Nil(t, err)
		node.signer = signer
		cfg := node.clientConfig()
		assert.Equal(t, 2, len(cfg.Auth))
		assert.True(t, strings.Contains(reflect.TypeOf(cfg.Auth[0]).String(), "password"),
			"first auth method should be password type, got %s", reflect.TypeOf(cfg.Auth[0]))
		assert.True(t, strings.Contains(reflect.TypeOf(cfg.Auth[1]).String(), "publicKey"),
			"second auth method should be publicKey type, got %s", reflect.TypeOf(cfg.Auth[1]))
	})
}
