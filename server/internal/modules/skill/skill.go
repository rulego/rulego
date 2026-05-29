package skill

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/server/app"
	"github.com/rulego/rulego/server/config"
	"github.com/rulego/rulego/server/model"
	"github.com/rulego/rulego/server/services"
	"github.com/rulego/rulego/utils/fs"
)

const (
	ModuleName = "skill"
	Priority   = 55
)

// Module skill 业务模块，负责 AI 技能管理。
type Module struct {
	cfg    *config.Config
	logger types.Logger
}

// New 创建 skill 模块
func New() *Module {
	return &Module{}
}

func (m *Module) Name() string  { return ModuleName }
func (m *Module) Priority() int { return Priority }

func (m *Module) Init(ctx *app.ModuleContext) error {
	m.cfg = ctx.Config
	m.logger = ctx.Logger

	globalSkillPath := m.getGlobalSkillPath()
	if err := fs.CreateDirs(globalSkillPath); err != nil {
		return fmt.Errorf("failed to create global skill directory: %v", err)
	}

	if err := ctx.Container.Register(services.KeySkillService, services.SkillService(m)); err != nil {
		return err
	}
	return nil
}

func (m *Module) Start(_ context.Context) error { return nil }
func (m *Module) Stop(_ context.Context) error  { return nil }

func getConfiguredGlobalSkillPath(skillPath string) string {
	skillPath = strings.TrimSpace(skillPath)
	if skillPath == "" {
		return "./skills"
	}
	return skillPath
}

func (m *Module) getGlobalSkillPath() string {
	if m.cfg == nil {
		return getConfiguredGlobalSkillPath("")
	}
	return getConfiguredGlobalSkillPath(m.cfg.SkillPath)
}

func (m *Module) getLocalSkillPath(username string) string {
	if m.cfg.SkillPath != "" {
		return m.cfg.SkillPath
	}
	return filepath.Join(m.cfg.DataDir, "workflows", username, "skills")
}

func (m *Module) ListSkills(username, scope string) ([]model.Skill, error) {
	var skills []model.Skill

	if scope == "global" || scope == "all" {
		globalSkills, err := m.listSkillsFromPath(m.getGlobalSkillPath(), "global")
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("Failed to list global skills: %v", err)
			}
		} else {
			skills = append(skills, globalSkills...)
		}
	}

	if scope == "local" || scope == "all" {
		localPath := m.getLocalSkillPath(username)
		localSkills, err := m.listSkillsFromPath(localPath, "local")
		if err != nil {
			if m.logger != nil {
				m.logger.Warnf("Failed to list local skills: %v", err)
			}
		} else {
			skills = append(skills, localSkills...)
		}
	}

	return skills, nil
}

func (m *Module) listSkillsFromPath(dirPath, scope string) ([]model.Skill, error) {
	var skills []model.Skill

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return skills, nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		skillPath := filepath.Join(dirPath, skillName)
		skillFile := filepath.Join(skillPath, "SKILL.md")

		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			continue
		}

		skill, err := m.parseSkillFile(skillFile, skillName, scope)
		if err != nil {
			if m.logger != nil {
				m.logger.Printf("Failed to parse skill %s: %v", skillName, err)
			}
			continue
		}

		skills = append(skills, *skill)
	}

	return skills, nil
}

func (m *Module) parseSkillFile(filePath, name, scope string) (*model.Skill, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	modTime := info.ModTime().Format(time.RFC3339)
	createTime := modTime

	frontmatter, body := parseFrontmatter(string(content))

	if frontmatter.Name == "" {
		frontmatter = parseFromContent(body)
	}

	skillName := frontmatter.Name
	if skillName == "" {
		skillName = name
	}

	return &model.Skill{
		Name:        skillName,
		Description: frontmatter.Description,
		Content:     string(content),
		Path:        filepath.Dir(filePath),
		Scope:       scope,
		CreatedAt:   createTime,
		UpdatedAt:   modTime,
	}, nil
}

func parseFrontmatter(content string) (model.FrontMatter, string) {
	var fm model.FrontMatter

	re := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n(.*)`)
	matches := re.FindStringSubmatch(content)

	if len(matches) == 3 {
		fmStr := matches[1]
		body := matches[2]

		lines := strings.Split(fmStr, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "name:") {
				fm.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				fm.Name = strings.Trim(fm.Name, `"`)
			} else if strings.HasPrefix(line, "description:") {
				fm.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				fm.Description = strings.Trim(fm.Description, `"`)
			}
		}

		return fm, body
	}

	return fm, content
}

func parseFromContent(content string) model.FrontMatter {
	var fm model.FrontMatter
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			fm.Name = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "> ") {
			fm.Description = strings.TrimSpace(strings.TrimPrefix(line, "> "))
			break
		}
	}

	return fm
}

func (m *Module) GetSkill(username, name, scope string) (*model.Skill, error) {
	var basePath string
	if scope == "global" {
		basePath = m.getGlobalSkillPath()
	} else {
		basePath = m.getLocalSkillPath(username)
	}

	skillFile := filepath.Join(basePath, name, "SKILL.md")
	if _, err := os.Stat(skillFile); os.IsNotExist(err) {
		return nil, errors.New("skill not found")
	}

	return m.parseSkillFile(skillFile, name, scope)
}

func (m *Module) CreateSkill(username string, skill model.Skill) (*model.Skill, error) {
	if err := validateSkillName(skill.Name); err != nil {
		return nil, err
	}

	var basePath string
	if skill.Scope == "global" {
		basePath = m.getGlobalSkillPath()
	} else {
		basePath = m.getLocalSkillPath(username)
	}

	skillDir := filepath.Join(basePath, skill.Name)
	if err := fs.CreateDirs(skillDir); err != nil {
		return nil, fmt.Errorf("failed to create skill directory: %v", err)
	}

	content := generateSkillContent(skill.Name, skill.Description, skill.Content)

	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write skill file: %v", err)
	}

	return m.parseSkillFile(skillFile, skill.Name, skill.Scope)
}

func (m *Module) UpdateSkill(username, name string, skill model.Skill) (*model.Skill, error) {
	existing, err := m.GetSkill(username, name, skill.Scope)
	if err != nil {
		return nil, err
	}

	var basePath string
	if skill.Scope == "global" {
		basePath = m.getGlobalSkillPath()
	} else {
		basePath = m.getLocalSkillPath(username)
	}

	content := generateSkillContent(existing.Name, skill.Description, skill.Content)

	skillFile := filepath.Join(basePath, name, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to update skill file: %v", err)
	}

	return m.parseSkillFile(skillFile, name, skill.Scope)
}

func (m *Module) DeleteSkill(username, name, scope string) error {
	var basePath string
	if scope == "global" {
		basePath = m.getGlobalSkillPath()
	} else {
		basePath = m.getLocalSkillPath(username)
	}

	skillDir := filepath.Join(basePath, name)
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return errors.New("skill not found")
	}

	return os.RemoveAll(skillDir)
}

// ImportSkills extracts SKILL.md files from a zip archive into the target
// scope directory and returns the imported skill metadata.
func (m *Module) ImportSkills(username, scope string, archive []byte) ([]model.Skill, error) {
	basePath := m.getSkillBasePath(username, scope)
	if err := fs.CreateDirs(basePath); err != nil {
		return nil, fmt.Errorf("failed to create skill directory: %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("failed to read zip archive: %v", err)
	}

	imported := make([]model.Skill, 0)
	seen := make(map[string]struct{})
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || pathpkg.Base(file.Name) != "SKILL.md" {
			continue
		}

		skillName := pathpkg.Base(pathpkg.Dir(file.Name))
		if err := validateSkillName(skillName); err != nil {
			return nil, err
		}
		content, err := readZipFile(file)
		if err != nil {
			return nil, err
		}
		skillDir := filepath.Join(basePath, skillName)
		if err := fs.CreateDirs(skillDir); err != nil {
			return nil, fmt.Errorf("failed to create skill directory: %v", err)
		}
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillFile, content, 0644); err != nil {
			return nil, fmt.Errorf("failed to write imported skill file: %v", err)
		}
		if _, ok := seen[skillName]; ok {
			continue
		}
		seen[skillName] = struct{}{}
		skill, err := m.parseSkillFile(skillFile, skillName, scope)
		if err != nil {
			return nil, err
		}
		imported = append(imported, *skill)
	}
	if len(imported) == 0 {
		return nil, errors.New("no SKILL.md files found in archive")
	}
	return imported, nil
}

func (m *Module) CopySkill(username, name, targetScope, newName string) (*model.Skill, error) {
	source, err := m.GetSkill(username, name, "global")
	if err != nil {
		return nil, err
	}

	if newName == "" {
		newName = name
	}

	newSkill := model.Skill{
		Name:        newName,
		Description: source.Description,
		Content:     extractBody(source.Content),
		Scope:       targetScope,
	}

	return m.CreateSkill(username, newSkill)
}

// getSkillBasePath resolves the storage root for the requested scope.
func (m *Module) getSkillBasePath(username, scope string) string {
	if scope == "global" {
		return m.getGlobalSkillPath()
	}
	return m.getLocalSkillPath(username)
}

// NormalizeSkillScope normalizes API-facing skill scopes.
func NormalizeSkillScope(scope string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "", "global":
		return "global", nil
	default:
		return "", errors.New("unsupported skill scope, only global is allowed")
	}
}

// normalizeSkillScope keeps package-local callers and tests aligned with the
// exported normalization helper.
func normalizeSkillScope(scope string) (string, error) {
	return NormalizeSkillScope(scope)
}

func validateSkillName(name string) error {
	if name == "" {
		return errors.New("skill name cannot be empty")
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
	if !matched {
		return errors.New("skill name can only contain letters, numbers, hyphens, and underscores")
	}
	return nil
}

func generateSkillContent(name, description, body string) string {
	if strings.HasPrefix(strings.TrimSpace(body), "---") {
		return body
	}
	return fmt.Sprintf(`---
name: %s
description: "%s"
---

%s`, name, description, body)
}

func extractBody(content string) string {
	_, body := parseFrontmatter(content)
	return body
}

// readZipFile returns the full content of a zip file entry.
func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open zip entry %s: %v", file.Name, err)
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read zip entry %s: %v", file.Name, err)
	}
	return content, nil
}
