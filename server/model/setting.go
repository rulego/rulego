package model

// UserSetting 用户设置领域模型
type UserSetting struct {
	LatestChainId string `json:"latestChainId"`
	CoreChainId   string `json:"coreChainId"`
}
