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

package rulego

import (
	"context"
	"fmt"
	"github.com/rulego/rulego/api/types"
	"sync"
)

type RelationCache struct {
	//入接点
	inNodeId types.RuleNodeId
	//与出节点连接关系
	relationType string
}

// RuleChainCtx 规则链实例定义
// 初始化所有节点
// 记录规则链，所有节点路由关系
type RuleChainCtx struct {
	//节点ID
	Id types.RuleNodeId
	//规则链定义
	SelfDefinition *RuleChain
	//规则引擎配置
	Config types.Config
	//是否已经初始化
	initialized bool
	//组件库
	componentsRegistry types.ComponentRegistry
	//节点ID列表
	nodeIds []types.RuleNodeId
	//组件列表
	nodes map[types.RuleNodeId]types.NodeCtx
	//组件路由关系
	nodeRoutes    map[types.RuleNodeId][]types.RuleNodeRelation
	nodeCtxRoutes map[types.RuleNodeId][]types.NodeCtx
	//通过入节点查询指定关系出节点列表缓存
	relationCache map[RelationCache][]types.NodeCtx
	//根上下文
	rootRuleContext types.RuleContext
	//子规则链池
	ruleChainPool *RuleGo
	//重新加载增强点切面
	reloadAspects []types.OnReloadAspect
	//销毁增强点切面
	destroyAspects []types.OnDestroyAspect
	sync.RWMutex
}

// InitRuleChainCtx 初始化RuleChainCtx
func InitRuleChainCtx(config types.Config, ruleChainDef *RuleChain) (*RuleChainCtx, error) {
	var ruleChainCtx = &RuleChainCtx{
		Config:             config,
		SelfDefinition:     ruleChainDef,
		nodes:              make(map[types.RuleNodeId]types.NodeCtx),
		nodeRoutes:         make(map[types.RuleNodeId][]types.RuleNodeRelation),
		relationCache:      make(map[RelationCache][]types.NodeCtx),
		componentsRegistry: config.ComponentsRegistry,
		initialized:        true,
	}
	if ruleChainDef.RuleChain.ID != "" {
		ruleChainCtx.Id = types.RuleNodeId{Id: ruleChainDef.RuleChain.ID, Type: types.CHAIN}
	}
	nodeLen := len(ruleChainDef.Metadata.Nodes)
	ruleChainCtx.nodeIds = make([]types.RuleNodeId, nodeLen)
	//加载所有节点信息
	for index, item := range ruleChainDef.Metadata.Nodes {
		if item.Id == "" {
			item.Id = fmt.Sprintf(defaultNodeIdPrefix+"%d", index)
		}
		ruleNodeId := types.RuleNodeId{Id: item.Id, Type: types.NODE}
		ruleChainCtx.nodeIds[index] = ruleNodeId
		ruleNodeCtx, err := InitRuleNodeCtx(config, item)
		if err != nil {
			return nil, err
		}
		ruleChainCtx.nodes[ruleNodeId] = ruleNodeCtx
	}
	//加载节点关系信息
	for _, item := range ruleChainDef.Metadata.Connections {
		inNodeId := types.RuleNodeId{Id: item.FromId, Type: types.NODE}
		outNodeId := types.RuleNodeId{Id: item.ToId, Type: types.NODE}
		ruleNodeRelation := types.RuleNodeRelation{
			InId:         inNodeId,
			OutId:        outNodeId,
			RelationType: item.Type,
		}
		nodeRelations, ok := ruleChainCtx.nodeRoutes[inNodeId]

		if ok {
			nodeRelations = append(nodeRelations, ruleNodeRelation)
		} else {
			nodeRelations = []types.RuleNodeRelation{ruleNodeRelation}
		}
		ruleChainCtx.nodeRoutes[inNodeId] = nodeRelations
	}
	//加载子规则链
	for _, item := range ruleChainDef.Metadata.RuleChainConnections {
		inNodeId := types.RuleNodeId{Id: item.FromId, Type: types.NODE}
		outNodeId := types.RuleNodeId{Id: item.ToId, Type: types.CHAIN}
		ruleChainRelation := types.RuleNodeRelation{
			InId:         inNodeId,
			OutId:        outNodeId,
			RelationType: item.Type,
		}

		nodeRelations, ok := ruleChainCtx.nodeRoutes[inNodeId]
		if ok {
			nodeRelations = append(nodeRelations, ruleChainRelation)
		} else {
			nodeRelations = []types.RuleNodeRelation{ruleChainRelation}
		}
		ruleChainCtx.nodeRoutes[inNodeId] = nodeRelations
	}

	if firstNode, ok := ruleChainCtx.GetFirstNode(); ok {
		ruleChainCtx.rootRuleContext = NewRuleContext(context.TODO(), ruleChainCtx.Config, ruleChainCtx, nil,
			firstNode, config.Pool, nil, nil)
	}

	//get aspects
	_, reloadAspects, destroyAspects := config.GetEngineAspects()
	ruleChainCtx.reloadAspects = reloadAspects
	ruleChainCtx.destroyAspects = destroyAspects
	return ruleChainCtx, nil
}

func (rc *RuleChainCtx) GetNodeById(id types.RuleNodeId) (types.NodeCtx, bool) {
	rc.RLock()
	defer rc.RUnlock()
	if id.Type == types.CHAIN {
		//子规则链通过规则链池查找
		if subRuleEngine, ok := rc.GetRuleChainPool().Get(id.Id); ok && subRuleEngine.rootRuleChainCtx != nil {
			return subRuleEngine.rootRuleChainCtx, true
		} else {
			return nil, false
		}
	} else {
		ruleNodeCtx, ok := rc.nodes[id]
		return ruleNodeCtx, ok
	}

}

func (rc *RuleChainCtx) GetNodeByIndex(index int) (types.NodeCtx, bool) {
	if index >= len(rc.nodeIds) {
		return &RuleNodeCtx{}, false
	}
	return rc.GetNodeById(rc.nodeIds[index])
}

// GetFirstNode 获取第一个节点，消息从该节点开始流转。默认是index=0的节点
func (rc *RuleChainCtx) GetFirstNode() (types.NodeCtx, bool) {
	var firstNodeIndex = rc.SelfDefinition.Metadata.FirstNodeIndex
	return rc.GetNodeByIndex(firstNodeIndex)
}

func (rc *RuleChainCtx) GetNodeRoutes(id types.RuleNodeId) ([]types.RuleNodeRelation, bool) {
	rc.RLock()
	defer rc.RUnlock()
	relations, ok := rc.nodeRoutes[id]
	return relations, ok
}

// GetNextNodes 获取当前节点指定关系的子节点
func (rc *RuleChainCtx) GetNextNodes(id types.RuleNodeId, relationType string) ([]types.NodeCtx, bool) {
	var nodeCtxList []types.NodeCtx
	cacheKey := RelationCache{inNodeId: id, relationType: relationType}
	rc.RLock()
	//get from cache
	nodeCtxList, ok := rc.relationCache[cacheKey]
	rc.RUnlock()
	if ok {
		return nodeCtxList, nodeCtxList != nil
	}

	//get from the Routes
	relations, ok := rc.GetNodeRoutes(id)
	hasNextComponents := false
	if ok {
		for _, item := range relations {
			if item.RelationType == relationType {
				if nodeCtx, nodeCtxOk := rc.GetNodeById(item.OutId); nodeCtxOk {
					nodeCtxList = append(nodeCtxList, nodeCtx)
					hasNextComponents = true
				}
			}
		}
	}
	rc.Lock()
	//add to the cache
	rc.relationCache[cacheKey] = nodeCtxList
	rc.Unlock()
	return nodeCtxList, hasNextComponents
}

// Type 组件类型
func (rc *RuleChainCtx) Type() string {
	return "ruleChain"
}

func (rc *RuleChainCtx) New() types.Node {
	panic("not support this func")
}

// Init 初始化
func (rc *RuleChainCtx) Init(_ types.Config, configuration types.Configuration) error {
	if rootRuleChainDef, ok := configuration["selfDefinition"]; ok {
		if v, ok := rootRuleChainDef.(*RuleChain); ok {
			if ruleChainCtx, err := InitRuleChainCtx(rc.Config, v); err == nil {
				rc.Copy(ruleChainCtx)
			} else {
				return err
			}
		}
	}
	return nil
	//return errors.New("not support this func")
}

// OnMsg 处理消息
func (rc *RuleChainCtx) OnMsg(ctx types.RuleContext, msg types.RuleMsg) {
	ctx.TellFlow(msg, rc.Id.Id, nil, nil)
}

func (rc *RuleChainCtx) Destroy() {
	rc.RLock()
	defer rc.RUnlock()
	for _, v := range rc.nodes {
		temp := v
		temp.Destroy()
	}
	//执行销毁切面逻辑
	for _, aop := range rc.destroyAspects {
		aop.OnDestroy(rc)
	}
}

func (rc *RuleChainCtx) IsDebugMode() bool {
	return rc.SelfDefinition.RuleChain.DebugMode
}

func (rc *RuleChainCtx) GetNodeId() types.RuleNodeId {
	return rc.Id
}

func (rc *RuleChainCtx) ReloadSelf(def []byte) error {
	var err error
	var ctx types.Node
	if ctx, err = rc.Config.Parser.DecodeRuleChain(rc.Config, def); err == nil {
		rc.Destroy()
		rc.Copy(ctx.(*RuleChainCtx))
	}
	//执行reload切面
	for _, aop := range rc.reloadAspects {
		aop.OnReload(rc, rc, err)
	}
	return err
}

func (rc *RuleChainCtx) ReloadChild(ruleNodeId types.RuleNodeId, def []byte) error {
	if node, ok := rc.GetNodeById(ruleNodeId); ok {
		//更新子节点
		err := node.ReloadSelf(def)
		//执行reload切面
		for _, aop := range rc.reloadAspects {
			aop.OnReload(rc, node, err)
		}
		return err
	}
	return nil
}

func (rc *RuleChainCtx) DSL() []byte {
	v, _ := rc.Config.Parser.EncodeRuleChain(rc.SelfDefinition)
	return v
}

// Copy 复制
func (rc *RuleChainCtx) Copy(newCtx *RuleChainCtx) {
	rc.Lock()
	defer rc.Unlock()
	rc.Id = newCtx.Id
	rc.Config = newCtx.Config
	rc.initialized = newCtx.initialized
	rc.componentsRegistry = newCtx.componentsRegistry
	rc.SelfDefinition = newCtx.SelfDefinition
	rc.nodeIds = newCtx.nodeIds
	rc.nodes = newCtx.nodes
	rc.nodeRoutes = newCtx.nodeRoutes
	rc.rootRuleContext = newCtx.rootRuleContext
	rc.ruleChainPool = newCtx.ruleChainPool
	rc.reloadAspects = newCtx.reloadAspects
	rc.destroyAspects = newCtx.destroyAspects
	//清除缓存
	rc.relationCache = make(map[RelationCache][]types.NodeCtx)
}

// SetRuleChainPool 设置子规则链池
func (rc *RuleChainCtx) SetRuleChainPool(ruleChainPool *RuleGo) {
	rc.ruleChainPool = ruleChainPool
}

// GetRuleChainPool 获取子规则链池
func (rc *RuleChainCtx) GetRuleChainPool() *RuleGo {
	if rc.ruleChainPool == nil {
		return DefaultRuleGo
	} else {
		return rc.ruleChainPool
	}
}
