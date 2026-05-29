package model

// Skill 技能结构
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Path        string `json:"path"`
	Scope       string `json:"scope"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// FrontMatter Markdown frontmatter
type FrontMatter struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}
