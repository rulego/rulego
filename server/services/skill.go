package services

import (
	"github.com/rulego/rulego/server/model"
)

// SkillService 技能服务接口
type SkillService interface {
	ListSkills(username, scope string) ([]model.Skill, error)
	GetSkill(username, name, scope string) (*model.Skill, error)
	CreateSkill(username string, skill model.Skill) (*model.Skill, error)
	UpdateSkill(username, name string, skill model.Skill) (*model.Skill, error)
	DeleteSkill(username, name, scope string) error
	CopySkill(username, name, targetScope, newName string) (*model.Skill, error)
	ImportSkills(username, scope string, archive []byte) ([]model.Skill, error)
}
