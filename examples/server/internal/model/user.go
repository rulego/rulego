package model

// User
type User struct {
	// Username
	Username string `json:"username"`
	// Password
	Password string `json:"password"`
	// Access Key
	ApiKey string `json:"apiKey"`
}
