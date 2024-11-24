package model

import "examples/server/internal/constants"

// UserSetting 用户设置
type UserSetting struct {
	// 最后修改规则链ID
	LatestChainId string `json:"latestChainId"`
	// 默认规则链ID，server所有事件都会发送至此
	CoreChainId string `json:"coreChainId"`
}

func (s UserSetting) GetLatestChainIdKey() string {
	return constants.SettingKeyLatestChainId
}
func (s UserSetting) GetCoreChainIdKey() string {
	return constants.SettingKeyCoreChainId
}
