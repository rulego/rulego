package endpoint

import (
	"strings"
	"testing"
)

func dsl(name string, nodes []string, conns int) []byte {
	var sb strings.Builder
	sb.WriteString(`{"ruleChain":{"id":"c","name":` + quote(name) + `},"metadata":{"nodes":[`)
	for i, n := range nodes {
		if i > 0 {
			sb.WriteString(",")
		}
		// n 形如 "id|name|config"
		parts := strings.SplitN(n, "|", 3)
		sb.WriteString(`{"id":` + quote(parts[0]) + `,"name":` + quote(parts[1]) + `,"configuration":` + parts[2] + `}`)
	}
	sb.WriteString(`],"connections":[`)
	for i := 0; i < conns; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"fromId":"a","toId":"b"}`)
	}
	sb.WriteString(`]}}`)
	return []byte(sb.String())
}

func quote(s string) string {
	return `"` + s + `"`
}

func TestRuleChangeSummary(t *testing.T) {
	oldDef := dsl("链A", []string{"n1|脚本|{\"v\":1}", "n2|HTTP|{}"}, 2)

	cases := []struct {
		name    string
		oldDef  []byte
		newDef  []byte
		contain string
	}{
		{"新建", nil, dsl("链A", []string{"n1|脚本|{}"}, 0), "新建规则链，包含 1 个节点"},
		{"无变化不生成明细", oldDef, oldDef, ""},
		{"加节点", oldDef, dsl("链A", []string{"n1|脚本|{\"v\":1}", "n2|HTTP|{}", "n3|延时|{}"}, 2), "添加节点 延时"},
		{"删节点", oldDef, dsl("链A", []string{"n1|脚本|{\"v\":1}"}, 1), "删除节点 HTTP"},
		{"改配置", oldDef, dsl("链A", []string{"n1|脚本|{\"v\":2}", "n2|HTTP|{}"}, 2), "脚本：配置 v 由 1 改为 2"},
		{"新增字段", oldDef, dsl("链A", []string{"n1|脚本|{\"v\":1,\"tag\":\"s\"}", "n2|HTTP|{}"}, 2), "新增配置 tag = s"},
		{"删字段", oldDef, dsl("链A", []string{"n1|脚本|{}", "n2|HTTP|{}"}, 2), "删除配置 v（原值 1）"},
		{"改节点名", oldDef, dsl("链A", []string{"n1|脚本2|{\"v\":1}", "n2|HTTP|{}"}, 2), `【脚本】重命名为【脚本2】`},
		{"键序不同不算修改", oldDef, dsl("链A", []string{"n1|脚本|{\"v\": 1}", "n2|HTTP|{}"}, 2), ""},
		{"链改名", oldDef, dsl("链B", []string{"n1|脚本|{\"v\":1}", "n2|HTTP|{}"}, 2), `规则链【链A】改名为【链B】`},
		{"连线变化", oldDef, dsl("链A", []string{"n1|脚本|{\"v\":1}", "n2|HTTP|{}"}, 1), "调整了连接关系（2 条改为 1 条）"},
		{"多字段折叠", oldDef, dsl("链A", []string{"n1|脚本|{\"a\":1,\"b\":2,\"c\":3,\"d\":4}", "n2|HTTP|{}"}, 2), "…等 5 处变更"},
		{"加节点折叠", oldDef, dsl("链A", []string{"n1|脚本|{\"v\":1}", "a|1|{}", "b|2|{}", "c|3|{}", "d|4|{}"}, 2), "添加节点 1、2、3 等 4 个"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ruleChangeSummary(c.oldDef, c.newDef)
			if c.contain == "" {
				if got != "" {
					t.Fatalf("want empty, got %q", got)
				}
				return
			}
			if !strings.Contains(got, c.contain) {
				t.Fatalf("got %q, want contain %q", got, c.contain)
			}
		})
	}

	// 坏 DSL 静默返回空
	if got := ruleChangeSummary([]byte("{bad"), []byte("{also bad")); got != "" {
		t.Fatalf("malformed dsl should be empty, got %q", got)
	}
}
