package filestore

import (
	"os"
	"path"

	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/utils/fs"
)

// NodePoolStore is implemented based on the node pool storage of the file system.
// Node pool data is stored in the user directory as JSON files.
type NodePoolStore struct {
	config   config.Config
	username string
}

// NewNodePoolStore creates node pool file storage
func NewNodePoolStore(cfg config.Config, username string) (*NodePoolStore, error) {
	return &NodePoolStore{
		config:   cfg,
		username: username,
	}, nil
}

// Get the node pool data
func (d *NodePoolStore) Get() ([]byte, error) {
	pathStr := d.getFilePath()
	if _, err := os.Stat(pathStr); os.IsNotExist(err) {
		return nil, nil
	}
	return os.ReadFile(pathStr)
}

// Save saves node pool data
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
