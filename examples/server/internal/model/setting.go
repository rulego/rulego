package model

// UserSetting
type UserSetting struct {
	// Finally, modify the rule chain ID
	LatestChainId string `json:"latestChainId"`
	// By default, the rule chain ID is sent, and all server events are sent here
	CoreChainId string `json:"coreChainId"`
}
