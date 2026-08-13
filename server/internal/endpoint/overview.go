package endpoint

import (
	"sort"
	"strings"
	"time"

	"github.com/rulego/rulego/api/types"
	endpointApi "github.com/rulego/rulego/api/types/endpoint"
	"github.com/rulego/rulego/endpoint"
	"github.com/rulego/rulego/server/internal/constants"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/utils/str"
)

// startTime 进程启动时刻，供 /version 返回。
var startTime = time.Now()

const (
	// overviewChainPageSize 概览统计拉取的链数量上限。
	overviewChainPageSize = 10000
	// overviewRunSampleSize 成功率采样的运行日志条数。run log 量级可能很大，只扫最近一页。
	overviewRunSampleSize = 200
	// overviewRecentErrorLimit 概览返回的最近错误条数上限。
	overviewRecentErrorLimit = 10
	// categoriesPageSize /rules/categories 拉取的链数量上限。
	categoriesPageSize = 10000
)

// chainStats 规则链维度统计。
type chainStats struct {
	Total    int `json:"total"`
	Deployed int `json:"deployed"`
	Disabled int `json:"disabled"`
	Root     int `json:"root"`
	Sub      int `json:"sub"`
}

// runStats 运行日志维度统计，基于最近 SampleSize 条采样。
type runStats struct {
	Total       int     `json:"total"`
	Success     int     `json:"success"`
	Failed      int     `json:"failed"`
	SuccessRate float64 `json:"successRate"`
	// SampleSize 实际参与成功率计算的条数；Total 是存储中的总条数。
	SampleSize int `json:"sampleSize"`
}

// recentError 概览里的最近错误摘要。
type recentError struct {
	Id        string `json:"id"`
	ChainId   string `json:"chainId"`
	ChainName string `json:"chainName"`
	ErrorMsg  string `json:"errorMsg"`
	EndTs     int64  `json:"endTs"`
}

// chainCategory 取规则链的 category，与 store 层一致走 AdditionalInfo。
func chainCategory(chain types.RuleChain) string {
	if cat, ok := chain.RuleChain.GetAdditionalInfo(constants.KeyCategory); ok {
		return strings.TrimSpace(str.ToString(cat))
	}
	return ""
}

// aggregateChains 按链列表算出各维度计数与去重排序后的 category 列表。
func aggregateChains(list []types.RuleChain, total int) (chainStats, []string) {
	stats := chainStats{Total: total}
	catSet := make(map[string]struct{})
	for _, c := range list {
		if c.RuleChain.Disabled {
			stats.Disabled++
		} else {
			stats.Deployed++
		}
		if c.RuleChain.Root {
			stats.Root++
		} else {
			stats.Sub++
		}
		if cat := chainCategory(c); cat != "" {
			catSet[cat] = struct{}{}
		}
	}
	return stats, sortedKeys(catSet)
}

// aggregateRuns 按最近 N 条运行日志算成功率与最近错误。
func aggregateRuns(events []model.Event, total int) (runStats, []recentError) {
	stats := runStats{Total: total, SampleSize: len(events)}
	errs := make([]recentError, 0, overviewRecentErrorLimit)
	for _, e := range events {
		if e.Success {
			stats.Success++
			continue
		}
		stats.Failed++
		if len(errs) < overviewRecentErrorLimit {
			errs = append(errs, recentError{
				Id:        e.Id,
				ChainId:   e.ChainId,
				ChainName: e.ChainName,
				ErrorMsg:  e.ErrorMsg,
				EndTs:     e.EndTs,
			})
		}
	}
	if stats.SampleSize > 0 {
		stats.SuccessRate = float64(stats.Success) / float64(stats.SampleSize)
	}
	return stats, errs
}

// sortedKeys 集合去重后排序输出。
func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (s *Server) registerOverviewRoutes(ep endpointApi.HttpEndpoint) {
	base := s.apiBasePath()

	// GET /overview 首页概览。所有统计都按当前登录用户聚合，不跨租户求和。
	ep.GET(endpoint.NewRouter().From(base + "/overview").Process(s.authWithPermission(constants.ResourceRule, "read")).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		username := metadataUsername(exchange)

		catalog, ok := getService[services.ChainCatalog](s, exchange, services.KeyRuleCatalog)
		if !ok {
			return false
		}
		list, total, err := catalog.List(username, "", nil, nil, "", overviewChainPageSize, 1)
		if err != nil {
			writeInternalError(exchange, err)
			return false
		}
		chains, categories := aggregateChains(list, total)

		// 运行统计只取最近一页采样，避免全量扫 run log。
		var runs runStats
		recentErrors := make([]recentError, 0, overviewRecentErrorLimit)
		if runLogSvc, err := getServiceRaw[services.RunLogService](s, services.KeyRunLogService); err == nil {
			events, runTotal, err := runLogSvc.List(username, "", time.Time{}, time.Time{}, overviewRunSampleSize, 1)
			if err == nil {
				runs, recentErrors = aggregateRuns(events, runTotal)
			}
		}

		writeJSON(exchange, map[string]interface{}{
			"chains":       chains,
			"runs":         runs,
			"recentErrors": recentErrors,
			"categories":   categories,
		})
		return true
	}).End())

	// GET /version 版本信息（任何已认证用户可看）。
	// 不返回 goVersion：该接口只认证不鉴权，Go 版本是运行时指纹，可据 CVE 查攻击面。
	ep.GET(endpoint.NewRouter().From(base + "/version").Process(s.authProcess()).Process(func(_ endpointApi.Router, exchange *endpointApi.Exchange) bool {
		writeJSON(exchange, map[string]interface{}{
			"version":    constants.ServerVersion,
			"apiVersion": apiVersion,
			"startTime":  startTime.Unix(),
		})
		return true
	}).End())
}
