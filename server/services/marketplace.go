package services

// MarketplaceService marketplace service interface
type MarketplaceService interface {
	GetComponents(checkMy bool) ([]interface{}, error)
	GetChains() ([]interface{}, error)
}
