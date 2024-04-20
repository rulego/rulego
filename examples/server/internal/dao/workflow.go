package dao

import (
	"examples/server/config"
	"examples/server/config/logger"
	"examples/server/internal/constants"
	"examples/server/internal/model"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

type WorkflowDao struct {
	Config config.Config
}

func NewWorkflowDao(config config.Config) (*WorkflowDao, error) {
	return &WorkflowDao{
		Config: config,
	}, nil
}

func (d *WorkflowDao) GetWorkflowPath(username, projectName string) string {
	return path.Join(d.Config.DataDir, username, projectName)
}

func (d *WorkflowDao) Create(project model.Workflow) error {
	workflowPath := d.GetWorkflowPath(project.Owner, project.Name)
	if err := os.Mkdir(workflowPath, os.ModePerm); err != nil {
		return err
	}
	now := strconv.FormatInt(time.Now().Unix(), 10)
	ruleChain := types.RuleChain{
		RuleChain: types.RuleChainBaseInfo{
			ID:   project.Name,
			Name: project.Name,
			AdditionalInfo: map[string]string{
				"createTime":  now,
				"updateTime":  now,
				"description": project.Description,
			},
		},
	}
	v, _ := json.Marshal(ruleChain)
	if err := fs.SaveFile(filepath.Join(workflowPath, project.Owner+"_"+project.Name), v); err != nil {
		logger.Logger.Printf("dao/workflow:Create save file error", err)
		return err
	}
	return nil
}

func (d *WorkflowDao) Delete(username, projectName string) error {
	return os.RemoveAll(d.GetWorkflowPath(username, projectName))
}

// List 获取用户下面所有项目
func (d *WorkflowDao) List(username string) []model.Workflow {
	userPath := path.Join(d.Config.DataDir, constants.WorkflowsDir, username)
	entries, err := os.ReadDir(userPath)
	if err != nil {
		return nil
	}
	var projects []model.Workflow
	for _, entry := range entries {
		if entry.IsDir() {
			dsl := fs.LoadFile(path.Join(userPath, entry.Name(), entry.Name()+".json"))
			var ruleChain types.RuleChain
			var createTime, updateTime int64
			var description string
			var additionalInfo map[string]string
			if err := json.Unmarshal(dsl, &ruleChain); err == nil {
				additionalInfo = ruleChain.RuleChain.AdditionalInfo
				if additionalInfo != nil {
					if createTimeStr, ok := additionalInfo["createTime"]; ok {
						createTime, _ = strconv.ParseInt(createTimeStr, 10, 64)
					}
					if updateTimeStr, ok := additionalInfo["updateTime"]; ok {
						updateTime, _ = strconv.ParseInt(updateTimeStr, 10, 64)
					}
					if descriptionStr, ok := additionalInfo["description"]; ok {
						description = descriptionStr
					}
				}
			}

			projects = append(projects, model.Workflow{
				Name:           entry.Name(),
				Owner:          username,
				Description:    description,
				CreateTime:     createTime,
				UpdateTime:     updateTime,
				AdditionalInfo: additionalInfo,
				RuleChain:      string(dsl),
			})
		}
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].CreateTime > projects[j].CreateTime
	})
	return projects
}
