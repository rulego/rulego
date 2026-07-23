package model

// Variable variable
type Variable struct {
	// Title
	Title string `json:"title"`
	// Name, unique
	Name string `json:"name"`
	// Content
	Value string `json:"value"`
	// Description
	Description string `json:"description"`
	// Type 0: Variable; 1: Confidential; 2: The key
	Type int `json:"type"`
	// Affiliated users
	Owner string `json:"owner"`
}
