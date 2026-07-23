package model

// UserSetting: Sets the domain model
type UserSetting struct {
	LatestChainId string `json:"latestChainId"`
	CoreChainId   string `json:"coreChainId"`
}
