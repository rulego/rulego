package dao

import (
	"examples/server/config"
	"examples/server/internal/constants"
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/json"
	"os"
	"path"
	"path/filepath"
)

type RuleDao struct {
	config config.Config
}

func NewRuleDao(config config.Config) (*RuleDao, error) {
	return &RuleDao{
		config: config,
	}, nil
}

func (d *RuleDao) Save(username, chainId string, def []byte) error {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule)
	pathStr := path.Join(paths...)
	//创建文件夹
	_ = fs.CreateDirs(pathStr)
	//保存到文件
	v, _ := json.Format(def)
	//保存规则链到文件
	return fs.SaveFile(filepath.Join(pathStr, chainId+constants.RuleChainFileSuffix), v)
}

func (d *RuleDao) Delete(username, chainId string) error {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule)
	pathStr := path.Join(paths...)
	file := filepath.Join(pathStr, chainId+constants.RuleChainFileSuffix)
	return os.RemoveAll(file)
}
