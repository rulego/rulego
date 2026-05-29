package services

// DebugService 调试服务接口
type DebugService interface {
	HandleDebugLog(username, chainId string, data map[string]interface{})
	SaveDebugData(username, chainId string, data map[string]interface{})
}
