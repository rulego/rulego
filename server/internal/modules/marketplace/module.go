// Package marketplace enables queries for remote component markets and rule chain markets.
package marketplace

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/services"
)

const (
	ModuleName = "marketplace"
	Priority   = 70
)

// MarketplaceResult: Remote market query results
type MarketplaceResult struct {
	Items []interface{} `json:"items"`
	Total int           `json:"total"`
	Page  int           `json:"page"`
	Size  int           `json:"size"`
}

// Module marketplace business module, responsible for remote component and rule chain market queries
type Module struct {
	cfg     *config.Config
	nodeSvc services.NodeService
}

// New to create the marketplace module
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	if err := ctx.Container.Register(services.KeyMarketplaceService, m); err != nil {
		return err
	}
	return nil
}

func (m *Module) Start(_ context.Context) error { return nil }
func (m *Module) Stop(_ context.Context) error  { return nil }

// SetNodeService Sets up node services (called by the endpoint layer)
func (m *Module) SetNodeService(svc services.NodeService) {
	m.nodeSvc = svc
}

// GetComponents retrieves the list of components, prioritizing remote markets; if not configured, it is obtained locally
func (m *Module) GetComponents(keywords string, page, size int) (*MarketplaceResult, error) {
	baseUrl := m.cfg.MarketplaceBaseUrl
	if baseUrl == "" {
		return m.getLocalComponents(keywords, page, size)
	}
	u := strings.TrimRight(baseUrl, "/") + "/marketplace/components"
	u = appendQueryParams(u, keywords, page, size, nil)
	return m.fetchList(u, page, size)
}

// GetChains obtains a list of rule chains, prioritizing remote markets; if not configured, they obtain locally
func (m *Module) GetChains(root *bool, keywords string, page, size int) (*MarketplaceResult, error) {
	baseUrl := m.cfg.MarketplaceBaseUrl
	if baseUrl == "" {
		return m.getLocalChains(keywords, page, size)
	}
	u := strings.TrimRight(baseUrl, "/") + "/marketplace/chains"
	u = appendQueryParams(u, keywords, page, size, root)
	return m.fetchList(u, page, size)
}

// getLocalComponents retrieves a list of components from the local area
func (m *Module) getLocalComponents(keywords string, page, size int) (*MarketplaceResult, error) {
	if m.nodeSvc == nil {
		return &MarketplaceResult{Items: []interface{}{}, Total: 0, Page: page, Size: size}, nil
	}
	// Use the default user admin to get the component
	ruleChains, total, err := m.nodeSvc.ListComponents("admin", keywords, size, page)
	if err != nil {
		return nil, err
	}
	items := make([]interface{}, len(ruleChains))
	for i, rc := range ruleChains {
		items[i] = rc
	}
	return &MarketplaceResult{Items: items, Total: total, Page: page, Size: size}, nil
}

// getLocalChains Retrieves the list of rule chains from the local area
func (m *Module) getLocalChains(keywords string, page, size int) (*MarketplaceResult, error) {
	// The local rule chain is managed by the rules module, which returns an empty list
	return &MarketplaceResult{Items: []interface{}{}, Total: 0, Page: page, Size: size}, nil
}

func appendQueryParams(rawURL, keywords string, page, size int, root *bool) string {
	params := url.Values{}
	if keywords != "" {
		params.Set("keywords", keywords)
	}
	params.Set("page", strconv.Itoa(page))
	params.Set("size", strconv.Itoa(size))
	if root != nil {
		params.Set("root", strconv.FormatBool(*root))
	}
	if len(params) > 0 {
		return rawURL + "?" + params.Encode()
	}
	return rawURL
}

func (m *Module) fetchList(rawURL string, defaultPage, defaultSize int) (*MarketplaceResult, error) {
	resp, err := http.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &url.Error{Op: "Get", URL: rawURL, Err: httpError(resp.StatusCode, body)}
	}

	// Try parsing it as a page map format {"total":..., "items":[...]}
	var mapResult map[string]interface{}
	if err := json.Unmarshal(body, &mapResult); err == nil {
		result := &MarketplaceResult{
			Page:  defaultPage,
			Size:  defaultSize,
			Total: toInt(mapResult["total"]),
		}
		if items, ok := mapResult["items"].([]interface{}); ok {
			result.Items = items
		} else if data, ok := mapResult["data"].([]interface{}); ok {
			result.Items = data
		}
		if v, ok := mapResult["page"].(float64); ok {
			result.Page = int(v)
		}
		if v, ok := mapResult["size"].(float64); ok {
			result.Size = int(v)
		}
		return result, nil
	}

	// Fallback: Parses as a pure array
	var list []interface{}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}
	return &MarketplaceResult{Items: list, Total: len(list), Page: defaultPage, Size: defaultSize}, nil
}

func toInt(v interface{}) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

type httpErr struct {
	status int
	body   string
}

func (e *httpErr) Error() string { return e.body }

func httpError(status int, body []byte) *httpErr {
	return &httpErr{status: status, body: string(body)}
}
