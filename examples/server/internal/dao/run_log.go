package dao

import (
	"examples/server/config"
	"examples/server/config/logger"
	"examples/server/internal/constants"
	"examples/server/internal/utils/file"
	"github.com/rulego/rulego/api/types"
	"github.com/rulego/rulego/utils/fs"
	"github.com/rulego/rulego/utils/json"
	"os"
	"path"
	"path/filepath"
	"time"
)

type EventDao struct {
	*FileStorage
	config config.Config
}

func NewEventDao(config config.Config) (*EventDao, error) {
	return &EventDao{
		config: config,
	}, nil
}

// SaveRunLog stores a snapshot of the workflow running log
func (s *EventDao) SaveRunLog(username string, ctx types.RuleContext, snapshot types.RuleChainRunSnapshot) error {
	var paths = []string{s.config.DataDir, constants.DirWorkflows}
	chainId := ctx.RuleChain().GetNodeId().Id
	paths = append(paths, username, constants.DirWorkflowsRun, chainId)
	pathStr := path.Join(paths...)
	//Create a folder
	_ = fs.CreateDirs(pathStr)
	snapshot.Id = time.Now().Format("20060102150405000") + "_" + snapshot.Id
	//Save to file
	if byteV, err := json.Marshal(snapshot); err != nil {
		logger.Logger.Printf("dao/EventDao:SaveRunLog marshal error: %v", err)
		return err
	} else {
		//v, _ := json.Format(byteV)
		//Save the rule chain to the file
		if err = fs.SaveFile(filepath.Join(pathStr, snapshot.Id), byteV); err != nil {
			logger.Logger.Printf("dao/EventDao:SaveRunLog save file error: %v", err)
			return err
		}
	}
	return nil
}

func (s *EventDao) Delete(username string, chainId, id string) error {
	var paths = []string{s.config.DataDir, constants.DirWorkflows}
	if id != "" {
		paths = append(paths, username, constants.DirWorkflowsRun, chainId, id)
	} else {
		paths = append(paths, username, constants.DirWorkflowsRun, chainId)
	}

	pathStr := path.Join(paths...)
	return os.RemoveAll(pathStr)
}

func (s *EventDao) DeleteByChainId(username string, chainId string) error {
	var paths = []string{s.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRun, chainId)
	pathStr := path.Join(paths...)
	return os.RemoveAll(pathStr)
}

func (s *EventDao) visit(files *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			*files = append(*files, path)
		}
		return nil
	}
}

func (s *EventDao) List(username string, chainId string, current, size int) ([]types.RuleChainRunSnapshot, int, error) {
	var snapshots []types.RuleChainRunSnapshot

	var paths = []string{s.config.DataDir, constants.DirWorkflows}
	if chainId == "" {
		//Load all
		paths = append(paths, username, constants.DirWorkflowsRun)
	} else {
		paths = append(paths, username, constants.DirWorkflowsRun, chainId)
	}
	pathStr := path.Join(paths...)
	// Retrieves all runtime log files in the directory
	var files []string
	if err := filepath.Walk(pathStr, s.visit(&files)); err != nil {
		return snapshots, 0, nil
	}
	// Sort by file timestamp
	fileWithTimestamps := file.SortFilesByTimestamp(files)

	// Calculate the starting index of the page
	start := (current - 1) * size
	end := start + size
	if end > len(files) {
		end = len(files)
	}

	// Traverse the file, with each file corresponding to a RuleChainRunSnapshot record
	for _, file := range fileWithTimestamps[start:end] {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return nil, 0, err
		}

		var snapshot types.RuleChainRunSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, 0, err
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots, len(files), nil
}

func (s *EventDao) Get(username, chainId, snapshotId string) (types.RuleChainRunSnapshot, error) {
	var snapshot types.RuleChainRunSnapshot
	var paths = []string{s.config.DataDir, constants.DirWorkflows}
	paths = append(paths, username, constants.DirWorkflowsRun, chainId, snapshotId)
	file := path.Join(paths...)
	data, err := os.ReadFile(file)
	if err != nil {
		return snapshot, err
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return snapshot, err
	} else {
		return snapshot, nil
	}
}
