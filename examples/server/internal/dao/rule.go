package dao

import (
	"examples/server/config"
	"examples/server/internal/constants"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/json"
	"github.com/rulego/rulego/utils/str"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type RuleDao struct {
	config config.Config
}

func NewRuleDao(config config.Config) (*RuleDao, error) {
	return &RuleDao{
		config: config,
	}, nil
}

// List chainType- 0: 规则链，1：根规则链，2：子规则链
func (d *RuleDao) List(username string, keywords string, chainType int, size, page int) ([]types.RuleChain, int, error) {
	var paths []string
	paths = append(paths, d.config.DataDir, constants.DirWorkflows)
	paths = append(paths, username, constants.DirWorkflowsRule)

	// 构建完整的路径
	basePath := filepath.Join(paths...)

	// 读取目录下的所有文件
	files, err := os.ReadDir(basePath)
	if err != nil {
		return nil, 0, err
	}

	var ruleChains []types.RuleChain
	totalCount := 0
	//是否搜索根规则链
	findRoot := chainType == 1
	// 遍历文件
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(strings.ToLower(file.Name())) == ".json" {
			// 构建文件的完整路径
			filePath := filepath.Join(basePath, file.Name())

			// 读取文件内容
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			// 解析 JSON 数据到 RuleChain 结构体
			var ruleChain types.RuleChain
			err = json.Unmarshal(data, &ruleChain)
			if err != nil {
				continue
			}
			if keywords == "" && (chainType == 0 || ruleChain.RuleChain.Root == findRoot) {
				ruleChains = append(ruleChains, ruleChain)
				totalCount++
			} else if keywords != "" &&
				(chainType == 0 || ruleChain.RuleChain.Root == findRoot) &&
				(strings.Contains(ruleChain.RuleChain.Name, keywords) || strings.Contains(ruleChain.RuleChain.ID, keywords)) {
				ruleChains = append(ruleChains, ruleChain)
				totalCount++
			}

		}
	}

	// 排序逻辑
	var updateTimeKey = "updateTime"
	sort.Slice(ruleChains, func(i, j int) bool {
		var iTime, jTime string
		if v, ok := ruleChains[i].RuleChain.GetAdditionalInfo(updateTimeKey); ok {
			iTime = str.ToString(v)
		}
		if v, ok := ruleChains[j].RuleChain.GetAdditionalInfo(updateTimeKey); ok {
			jTime = str.ToString(v)
		}
		return iTime > jTime
	})
	//page==0查所有
	if page == 0 {
		return ruleChains, totalCount, nil
	}
	// 根据分页参数调整返回的数据
	start := (page - 1) * size
	end := start + size
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}

	// 返回分页后的数据
	return ruleChains[start:end], totalCount, nil
}

func (d *RuleDao) Get(username, chainId string) ([]byte, error) {
	var paths = []string{d.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRule, chainId+constants.RuleChainFileSuffix)
	pathStr := path.Join(paths...)
	return os.ReadFile(pathStr)
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
