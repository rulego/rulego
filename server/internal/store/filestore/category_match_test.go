package filestore

import "testing"

// category 是路径式的（存储时按 "/" 分段建目录，见 isSafeCategory 允许 "a/b"）。
// 查询父级必须命中子级，否则树状导航点父节点会显示为空。
func TestCategoryMatches(t *testing.T) {
	cases := []struct {
		name  string
		item  string
		query string
		want  bool
	}{
		// 空查询 = 不过滤
		{"空查询命中一切", "collect/modbus", "", true},
		{"空查询命中无分类项", "", "", true},
		{"仅空格的查询等同空", "collect/modbus", "   ", true},

		// 精确匹配
		{"精确同级", "alarm", "alarm", true},
		{"精确多层", "collect/modbus", "collect/modbus", true},

		// 父级命中子级（本次修复的核心）
		{"父级命中子级", "collect/modbus", "collect", true},
		{"父级命中孙级", "collect/modbus/tcp", "collect", true},
		{"中间层命中孙级", "collect/modbus/tcp", "collect/modbus", true},

		// 边界：必须在 "/" 处切，不能裸前缀
		{"不误命中兄弟分类", "collection", "collect", false},
		{"不误命中同前缀长名", "collect-old/x", "collect", false},
		{"子级查询不命中父级", "collect", "collect/modbus", false},

		// 查询串两端斜杠规整
		{"查询带尾斜杠", "collect/modbus", "collect/", true},
		{"查询带首斜杠", "collect/modbus", "/collect", true},
		{"查询首尾都有斜杠", "collect/modbus", "/collect/", true},

		// 项的分类带斜杠也要规整（历史数据可能存成 "collect/"）
		{"项带尾斜杠仍精确命中", "collect/", "collect", true},

		// 无分类项
		{"无分类项不被具体查询命中", "", "collect", false},

		// 大小写：category 是用户自由输入，保持区分（与目录名一致）
		{"大小写敏感", "Collect/modbus", "collect", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := categoryMatches(c.item, c.query); got != c.want {
				t.Errorf("categoryMatches(%q, %q) = %v, 期望 %v", c.item, c.query, got, c.want)
			}
		})
	}
}
