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
	"errors"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/test"
	"github.com/rulego/rulego/test/assert"
	"golang.org/x/crypto/ssh"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSshNode(t *testing.T) {
	var targetNodeType = "ssh"

	serverIp := os.Getenv("TEST_SERVER_IP")
	serverPort := os.Getenv("TEST_SERVER_PORT")
	if serverPort == "" {
		serverPort = "22"
	}
	serverUsername := os.Getenv("TEST_SERVER_USERNAME")
	serverPassword := os.Getenv("TEST_SERVER_PASSWORD")

	t.Run("NewNode", func(t *testing.T) {
		test.NodeNew(t, targetNodeType, &SshNode{}, types.Configuration{
			"host":     "127.0.0.1",
			"port":     22,
			"username": "root",
			"password": "password",
		}, Registry)
	})

	port, err := strconv.Atoi(serverPort)
	if err != nil {
		port = 22
	}
	t.Run("InitNode", func(t *testing.T) {
		if serverIp == "" {
			return
		}
		test.NodeInit(t, targetNodeType, types.Configuration{
			"host":     serverIp,
			"port":     port,
			"username": serverUsername,
			"password": serverPassword,
			"cmd":      "echo test",
		}, types.Configuration{
			"host":     serverIp,
			"port":     22,
			"username": serverUsername,
			"password": serverPassword,
			"cmd":      "echo test",
		}, Registry)
	})

	t.Run("DefaultConfig", func(t *testing.T) {
		if serverIp == "" {
			return
		}
		test.NodeInit(t, targetNodeType, types.Configuration{
			"host":     serverIp,
			"port":     22,
			"username": serverUsername,
			"password": serverPassword,
			"cmd":      "echo test",
		}, types.Configuration{
			"host":     serverIp,
			"port":     22,
			"username": serverUsername,
			"password": serverPassword,
			"cmd":      "echo test",
		}, Registry)
	})

	t.Run("OnMsg", func(t *testing.T) {
		if serverIp == "" {
			return
		}
		node1, err := test.CreateAndInitNode(targetNodeType, types.Configuration{
			"host":     serverIp,
			"port":     port,
			"username": serverUsername,
			"password": serverPassword,
			"cmd":      "echo \"hello world\"",
		}, Registry)
		assert.Nil(t, err)

		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"host":     "127.0.0.1",
			"Port":     22,
			"username": "root",
			"password": "password",
		}, Registry)
		assert.NotNil(t, err)

		_, err = test.CreateAndInitNode(targetNodeType, types.Configuration{
			"host":     serverIp,
			"port":     port,
			"username": serverUsername,
			"password": serverPassword,
			"cmd":      "",
		}, Registry)
		assert.NotNil(t, err)

		node4 := &SshNode{}
		err = node4.Init(types.NewConfig(), types.Configuration{})
		assert.Equal(t, SshConfigEmptyErr.Error(), err.Error())
		ctx := test.NewRuleContextFull(types.NewConfig(), node4, nil, func(msg types.RuleMsg, relationType string, err error) {
			assert.Equal(t, SshClientNotInitErr.Error(), err.Error())
		})
		node4.OnMsg(ctx, types.RuleMsg{})

		metaData := types.BuildMetadata(make(map[string]string))
		metaData.PutValue("productType", "test")
		msgList := []test.Msg{
			{
				MetaData:   metaData,
				MsgType:    "ACTIVITY_EVENT2",
				Data:       "{\"temperature\":60}",
				AfterSleep: time.Millisecond * 200,
			},
		}

		var nodeList = []test.NodeAndCallback{
			{
				Node:    node1,
				MsgList: msgList,
				Callback: func(msg types.RuleMsg, relationType string, err error) {

					assert.True(t, strings.Contains(msg.GetData(), "hello world"))
					assert.Equal(t, types.Success, relationType)
				},
			},
		}
		for _, item := range nodeList {
			test.NodeOnMsgWithChildren(t, item.Node, item.MsgList, item.ChildrenNodes, item.Callback)
		}
	})
}

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
		{"plain PEM content", SshConfiguration{CertKeyFile: plainPEM}, false, false, ""},
		{"plain PEM with redundant passphrase", SshConfiguration{CertKeyFile: plainPEM, Password: "ignored"}, false, false, ""},
		{"plain PEM file", SshConfiguration{CertKeyFile: keyPath}, false, false, ""},
		{"encrypted with correct passphrase", SshConfiguration{CertKeyFile: encryptedPEM, Password: passphrase}, false, false, ""},
		{"encrypted with wrong passphrase", SshConfiguration{CertKeyFile: encryptedPEM, Password: "wrong-passphrase"}, false, true, ""},
		{"encrypted without passphrase", SshConfiguration{CertKeyFile: encryptedPEM}, false, true, "password (used as passphrase)"},
		{"missing key file", SshConfiguration{CertKeyFile: filepath.Join(dir, "no-such-file")}, false, true, "read private key file"},
		{"invalid PEM content", SshConfiguration{CertKeyFile: "-----BEGIN RSA PRIVATE KEY-----\ninvalid"}, false, true, "parse private key"},
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
			"host":        "127.0.0.1",
			"port":        22,
			"username":    "root",
			"certKeyFile": plainPEM,
			"cmd":         "ls",
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
			"host":        "127.0.0.1",
			"port":        22,
			"username":    "root",
			"certKeyFile": keyPath,
			"cmd":         "ls",
		})
		assert.Nil(t, err)
		assert.NotNil(t, node.signer)
		node.Destroy()
	})

	t.Run("password and certKeyFile are mutually exclusive", func(t *testing.T) {
		node := &SshNode{}
		err := node.Init(types.NewConfig(), types.Configuration{
			"host":        "127.0.0.1",
			"port":        22,
			"username":    "root",
			"password":    "secret",
			"certKeyFile": plainPEM,
			"cmd":         "ls",
		})
		assert.NotNil(t, err)
		assert.Equal(t, SshConfigPasswordKeyExclusiveErr.Error(), err.Error())
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
			"host":        "127.0.0.1",
			"port":        22,
			"username":    "root",
			"certKeyFile": "-----BEGIN RSA PRIVATE KEY-----\ngarbage",
			"cmd":         "ls",
		})
		assert.NotNil(t, err)
		assert.True(t, strings.Contains(err.Error(), "parse private key"))
	})
}

// TestSshNodeClientConfigAuth 覆盖 clientConfig 的认证方式组装。
// TestSshNodeClientConfigAuth covers auth method assembly in clientConfig.
func TestSshNodeClientConfigAuth(t *testing.T) {
	const passphrase = "test-passphrase"
	plainPEM, encryptedPEM := testKeyPair(t, passphrase)

	t.Run("password only", func(t *testing.T) {
		node := &SshNode{Config: SshConfiguration{Username: "root", Password: "secret"}}
		cfg := node.clientConfig()
		assert.Equal(t, 1, len(cfg.Auth))
		assert.True(t, strings.Contains(reflect.TypeOf(cfg.Auth[0]).String(), "password"),
			"auth method should be password type, got %s", reflect.TypeOf(cfg.Auth[0]))
	})

	t.Run("private key only", func(t *testing.T) {
		node := &SshNode{Config: SshConfiguration{Username: "root", CertKeyFile: plainPEM}}
		signer, err := node.parseSigner()
		assert.Nil(t, err)
		node.signer = signer
		cfg := node.clientConfig()
		assert.Equal(t, 1, len(cfg.Auth))
		// 无私钥时不放空密码认证 - no empty password auth when password is empty
		assert.True(t, strings.Contains(reflect.TypeOf(cfg.Auth[0]).String(), "publicKey"),
			"auth method should be publicKey type, got %s", reflect.TypeOf(cfg.Auth[0]))
	})

	t.Run("private key with password (passphrase) uses public key only", func(t *testing.T) {
		node := &SshNode{Config: SshConfiguration{Username: "root", Password: passphrase, CertKeyFile: encryptedPEM}}
		signer, err := node.parseSigner()
		assert.Nil(t, err)
		node.signer = signer
		cfg := node.clientConfig()
		assert.Equal(t, 1, len(cfg.Auth))
		assert.True(t, strings.Contains(reflect.TypeOf(cfg.Auth[0]).String(), "publicKey"),
			"auth method should be publicKey type, got %s", reflect.TypeOf(cfg.Auth[0]))
	})
}

func TestIsSshCmdError(t *testing.T) {
	assert.True(t, isSshCmdError(&ssh.ExitError{}))
	assert.True(t, isSshCmdError(fmt.Errorf("run: %w", &ssh.ExitMissingError{})))
	assert.False(t, isSshCmdError(errors.New("read tcp 1.2.3.4:22: connection reset")))
	assert.False(t, isSshCmdError(nil))
}

func TestSshNodeNotInit(t *testing.T) {
	node := &SshNode{}
	err := node.Init(types.NewConfig(), types.Configuration{})
	assert.Equal(t, SshConfigEmptyErr.Error(), err.Error())
	ctx := test.NewRuleContext(types.NewConfig(), func(msg types.RuleMsg, relationType string, err error) {
		assert.Equal(t, types.Failure, relationType)
		assert.Equal(t, SshClientNotInitErr.Error(), err.Error())
	})
	node.OnMsg(ctx, types.RuleMsg{})
}

func TestSshNodeConnectionStatusUnreachable(t *testing.T) {
	node := &SshNode{}
	config := types.NewConfig()
	// 懒初始化：目标机不可达不影响 Init
	err := node.Init(config, types.Configuration{
		"host":     "127.0.0.1",
		"port":     1,
		"username": "root",
		"password": "password",
		"cmd":      "echo test",
	})
	assert.Nil(t, err)
	assert.Equal(t, types.StatusNone, node.ConnectionStatus().Status)

	ctx := test.NewRuleContext(config, func(msg types.RuleMsg, relationType string, err error) {
		assert.Equal(t, types.Failure, relationType)
		assert.True(t, err != nil)
	})
	node.OnMsg(ctx, ctx.NewMsg("AA", types.NewMetadata(), ""))

	info := node.ConnectionStatus()
	assert.Equal(t, types.StatusReconnecting, info.Status)
	assert.True(t, info.Message != "")

	node.Destroy()
	assert.Equal(t, types.StatusDisconnected, node.ConnectionStatus().Status)
}
