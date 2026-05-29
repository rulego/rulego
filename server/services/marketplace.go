package services

// MarketplaceService 市场服务接口
type MarketplaceService interface {
	GetComponents(checkMy bool) ([]interface{}, error)
	GetChains() ([]interface{}, error)
}
