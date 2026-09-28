package endpoint

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// 规则链保存事件的变更摘要：对比保存前后 DSL，生成一行中文明细。
// 任何解析失败都返回空串，审计只缺明细不缺事件。

type ruleNode struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Configuration json.RawMessage `json:"configuration"`
}

type ruleDSL struct {
	RuleChain struct {
		Name string `json:"name"`
	} `json:"ruleChain"`
	Metadata struct {
		Nodes       []ruleNode        `json:"nodes"`
		Connections []json.RawMessage `json:"connections"`
	} `json:"metadata"`
}

// canonical 把 JSON 规范化后比较，规避键序与空白造成的假差异
func canonical(v json.RawMessage) string {
	if len(v) == 0 {
		return ""
	}
	var m interface{}
	if err := json.Unmarshal(v, &m); err != nil {
		return string(v)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return string(v)
	}
	return string(b)
}

func nodeLabel(n ruleNode) string {
	if n.Name != "" {
		return n.Name
	}
	return n.ID
}

// configChanges 逐字段对比节点配置，返回中文描述（如「配置 v 由 1 改为 2」）。
// 最多列 3 个字段，超出折叠为「…等其他 N 处变更」；值超 24 字符截断。
func configChanges(oldConf, newConf json.RawMessage) []string {
	om, nm := configMap(oldConf), configMap(newConf)
	type change struct {
		key     string
		oldV    interface{}
		newV    interface{}
		added   bool
		removed bool
	}
	var changes []change
	for k, nv := range nm {
		ov, existed := om[k]
		if !existed {
			changes = append(changes, change{k, nil, nv, true, false})
		} else if jsonBrief(ov) != jsonBrief(nv) {
			changes = append(changes, change{k, ov, nv, false, false})
		}
	}
	for k, ov := range om {
		if _, ok := nm[k]; !ok {
			changes = append(changes, change{k, ov, nil, false, true})
		}
	}
	if len(changes) == 0 {
		return nil
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].key < changes[j].key })

	var out []string
	for i, c := range changes {
		if i >= 3 {
			out = append(out, fmt.Sprintf("…等 %d 处变更", len(changes)))
			break
		}
		switch {
		case c.added:
			out = append(out, fmt.Sprintf("新增配置 %s = %s", c.key, jsonBrief(c.newV)))
		case c.removed:
			out = append(out, fmt.Sprintf("删除配置 %s（原值 %s）", c.key, jsonBrief(c.oldV)))
		default:
			out = append(out, fmt.Sprintf("配置 %s 由 %s 改为 %s", c.key, jsonBrief(c.oldV), jsonBrief(c.newV)))
		}
	}
	return out
}

func configMap(raw json.RawMessage) map[string]interface{} {
	m := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

// jsonBrief 值的紧凑渲染：字符串去引号，其余为紧凑 JSON，超 24 字符截断加省略号
func jsonBrief(v interface{}) string {
	if v == nil {
		return "空"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "?"
	}
	s := string(b)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	r := []rune(s)
	if len(r) > 24 {
		return string(r[:24]) + "…"
	}
	return s
}

// foldItems 折叠名词列表：最多展示 3 个，超出以「等 N 个」收尾
func foldItems(items []string) string {
	joined := items[0]
	for i := 1; i < len(items) && i < 3; i++ {
		joined += "、" + items[i]
	}
	if len(items) > 3 {
		joined += fmt.Sprintf(" 等 %d 个", len(items))
	}
	return joined
}

// 修改明细的展示上限：超出后不再逐条展开，只提示总量
const maxModifiedSegments = 4

func ruleChangeSummary(oldDef, newDef []byte) string {
	var newDSL ruleDSL
	if len(newDef) == 0 || json.Unmarshal(newDef, &newDSL) != nil {
		return ""
	}
	var oldDSL ruleDSL
	hasOld := len(oldDef) > 0 && json.Unmarshal(oldDef, &oldDSL) == nil
	if !hasOld {
		return fmt.Sprintf("新建规则链，包含 %d 个节点", len(newDSL.Metadata.Nodes))
	}

	oldNodes := make(map[string]ruleNode, len(oldDSL.Metadata.Nodes))
	for _, n := range oldDSL.Metadata.Nodes {
		oldNodes[n.ID] = n
	}
	newNodes := make(map[string]ruleNode, len(newDSL.Metadata.Nodes))
	for _, n := range newDSL.Metadata.Nodes {
		newNodes[n.ID] = n
	}

	var added, removed, renamed []string
	var modified []string
	for _, n := range newDSL.Metadata.Nodes {
		o, ok := oldNodes[n.ID]
		if !ok {
			added = append(added, nodeLabel(n))
			continue
		}
		if o.Name != n.Name && o.Name != "" && n.Name != "" {
			renamed = append(renamed, fmt.Sprintf("%q 重命名为 %q", o.Name, n.Name))
		}
		for _, c := range configChanges(o.Configuration, n.Configuration) {
			modified = append(modified, nodeLabel(n)+"："+c)
		}
	}
	for _, n := range oldDSL.Metadata.Nodes {
		if _, ok := newNodes[n.ID]; !ok {
			removed = append(removed, nodeLabel(n))
		}
	}

	var parts []string
	if oldName, newName := oldDSL.RuleChain.Name, newDSL.RuleChain.Name; newName != "" && oldName != newName {
		parts = append(parts, fmt.Sprintf("规则链 %q 改名为 %q", oldName, newName))
	}
	if len(added) > 0 {
		parts = append(parts, "添加节点 "+foldItems(added))
	}
	if len(renamed) > 0 {
		parts = append(parts, "节点 "+foldItems(renamed))
	}
	if len(modified) > 0 {
		if len(modified) > maxModifiedSegments {
			parts = append(parts, modified[:maxModifiedSegments]...)
			parts = append(parts, fmt.Sprintf("…另有 %d 处节点修改", len(modified)-maxModifiedSegments))
		} else {
			parts = append(parts, modified...)
		}
	}
	if len(removed) > 0 {
		parts = append(parts, "删除节点 "+foldItems(removed))
	}
	if oldC, newC := len(oldDSL.Metadata.Connections), len(newDSL.Metadata.Connections); oldC != newC {
		parts = append(parts, fmt.Sprintf("调整了连接关系（%d 条改为 %d 条）", oldC, newC))
	}

	return strings.Join(parts, "；")
}
