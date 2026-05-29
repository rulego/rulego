package filestore

import (
	"os"
	"path"

	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/utils/fs"
)

// NodePoolStore 基于文件系统的节点池存储实现。
// 节点池数据以 JSON 文件形式存储在用户目录下。
type NodePoolStore struct {
	config   config.Config
	username string
}

// NewNodePoolStore 创建节点池文件存储
func NewNodePoolStore(cfg config.Config, username string) (*NodePoolStore, error) {
	return &NodePoolStore{
		config:   cfg,
		username: username,
	}, nil
}

// Get 获取节点池数据
func (d *NodePoolStore) Get() ([]byte, error) {
	pathStr := d.getFilePath()
	if _, err := os.Stat(pathStr); os.IsNotExist(err) {
		return nil, nil
	}
	return os.ReadFile(pathStr)
}

// Save 保存节点池数据
func (d *NodePoolStore) Save(data []byte) error {
	pathStr := d.getFilePath()
	dir := path.Dir(pathStr)
	if err := fs.CreateDirs(dir); err != nil {
		return err
	}
	return fs.SaveFile(pathStr, data)
}

func (d *NodePoolStore) getFilePath() string {
	return path.Join(d.config.DataDir, constants.DirWorkflows, d.username, "node_pool.json")
}
