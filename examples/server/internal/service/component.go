/*
 * Copyright 2025 The RuleGo Authors.
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

package service

import (
	"errors"
	"examples/server/config"
	"examples/server/internal/constants"
	"examples/server/internal/dao"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/engine"
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/json"
	"path"
	"path/filepath"
)

// ComponentService 自定义组件服务
type ComponentService struct {
	username     string
	config       config.Config
	ruleConfig   types.Config
	componentDao *dao.ComponentDao
	mcpService   *McpService
}

func NewComponentService(ruleConfig types.Config, c config.Config, username string) (*ComponentService, error) {
	folderPath := path.Join(c.DataDir, constants.DirWorkflows, username, constants.DirWorkflowsComponent)
	_ = fs.CreateDirs(folderPath)

	componentDao, err := dao.NewComponentDao(c, username)
	if err != nil {
		return nil, err
	}
	return &ComponentService{
		username:     username,
		config:       c,
		ruleConfig:   ruleConfig,
		componentDao: componentDao,
	}, nil
}
func (s *ComponentService) GetRuleConfig() types.Config {
	return s.ruleConfig
}

func (s *ComponentService) LoadComponents() {
	folderPath := path.Join(s.config.DataDir, constants.DirWorkflows, s.username, constants.DirWorkflowsComponent)
	_ = fs.CreateDirs(folderPath)
	folderPath = folderPath + "/*.json"
	paths, err := fs.GetFilePaths(folderPath)
	if err != nil {
		return
	}
	for _, p := range paths {
		fileName := filepath.Base(p)
		chainId := fileName[:len(fileName)-len(filepath.Ext(fileName))]
		if def, err := s.componentDao.Get(s.username, chainId); err == nil {
			var ruleChain types.RuleChain

			if err = json.Unmarshal(def, &ruleChain); err != nil {
				continue
			}
			if err = s.ComponentsRegistry().Register(engine.NewDynamicNode(ruleChain.RuleChain.ID, string(def))); err != nil {
				s.ruleConfig.Logger.Printf("load component id=%s error: %s", ruleChain.RuleChain.ID, err.Error())
				return
			}
		}
	}
}

func (s *ComponentService) ComponentsRegistry() types.ComponentRegistry {
	return s.ruleConfig.ComponentsRegistry
}

func (s *ComponentService) List(keywords string, size, page int) ([]types.RuleChain, int, error) {
	return s.componentDao.List(s.username, keywords, nil, nil, size, page)
}

func (s *ComponentService) Get(nodeType string) ([]byte, error) {
	return s.componentDao.Get(s.username, nodeType)
}

func (s *ComponentService) Install(node types.Node) error {
	err := s.ComponentsRegistry().Register(node)
	if err != nil {
		return err
	}
	dynamicNode, ok := node.(*engine.DynamicNode)
	if ok {
		if err = s.componentDao.Save(s.username, node.Type(), []byte(dynamicNode.Dsl)); err != nil {
			return err
		} else {
			if s.mcpService != nil {
				s.mcpService.AddToolsFromComponent(node.Type(), dynamicNode.Def())
			}
			return nil
		}
	} else {
		return errors.New(fmt.Sprintf("node:%s 类型错误", node.Type()))
	}
}

func (s *ComponentService) Upgrade(node types.Node) error {
	_ = s.ComponentsRegistry().Unregister(node.Type())
	return s.Install(node)
}

func (s *ComponentService) Uninstall(nodeType string) error {
	if s.mcpService != nil {
		s.mcpService.DeleteTools(nodeType)
	}
	_ = s.ComponentsRegistry().Unregister(nodeType)
	return s.componentDao.Delete(s.username, nodeType)
}
