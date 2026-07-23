package services

// DebugService debugging service interface
type DebugService interface {
	HandleDebugLog(username, chainId string, data map[string]interface{})
	SaveDebugData(username, chainId string, data map[string]interface{})
}
