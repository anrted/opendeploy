package site

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/anrted/opendeploy/internal/platform/apperrors"
	"github.com/anrted/opendeploy/pkg/contract"
)

type FileService struct {
	repo  Repository
	agent contract.AgentClient
}

func NewFileService(repo Repository, agent contract.AgentClient) *FileService {
	return &FileService{repo: repo, agent: agent}
}

func (s *FileService) ensureAgent() error {
	if s.agent == nil {
		return fmt.Errorf("agent is unavailable")
	}
	return nil
}

func (s *FileService) resolvePath(ctx context.Context, siteID, relativePath string) (string, error) {
	current, err := s.repo.FindByID(ctx, siteID)
	if err != nil {
		return "", err
	}
	cleanRelative := path.Clean("/" + relativePath)
	absolutePath := path.Join(current.RootPath, cleanRelative)
	if absolutePath != current.RootPath && !strings.HasPrefix(absolutePath, current.RootPath+"/") {
		return "", apperrors.InvalidInput("invalid file path")
	}
	return absolutePath, nil
}

func (s *FileService) List(ctx context.Context, siteID, relativePath string) ([]contract.FileInfo, error) {
	absolutePath, err := s.resolvePath(ctx, siteID, relativePath)
	if err != nil {
		return nil, err
	}
	if s.agent == nil {
		return nil, fmt.Errorf("agent is unavailable")
	}
	return s.agent.DirList(ctx, absolutePath)
}

func (s *FileService) Read(ctx context.Context, siteID, relativePath string) ([]byte, error) {
	absolutePath, err := s.resolvePath(ctx, siteID, relativePath)
	if err != nil {
		return nil, err
	}
	if s.agent == nil {
		return nil, fmt.Errorf("agent is unavailable")
	}
	return s.agent.FileRead(ctx, absolutePath)
}

func (s *FileService) Write(ctx context.Context, siteID, relativePath string, content []byte) error {
	absolutePath, err := s.resolvePath(ctx, siteID, relativePath)
	if err != nil {
		return err
	}
	if s.agent == nil {
		return fmt.Errorf("agent is unavailable")
	}
	return s.agent.FileWrite(ctx, absolutePath, content, 0o644)
}

func (s *FileService) CreateDirectory(ctx context.Context, siteID, relativePath string) error {
	absolutePath, err := s.resolvePath(ctx, siteID, relativePath)
	if err != nil {
		return err
	}
	if s.agent == nil {
		return fmt.Errorf("agent is unavailable")
	}
	return s.agent.DirCreate(ctx, absolutePath, 0o755)
}

func (s *FileService) Delete(ctx context.Context, siteID, relativePath string) error {
	absolutePath, err := s.resolvePath(ctx, siteID, relativePath)
	if err != nil {
		return err
	}
	current, err := s.repo.FindByID(ctx, siteID)
	if err != nil {
		return err
	}
	if absolutePath == current.RootPath {
		return apperrors.InvalidInput("cannot delete site root directory")
	}
	if s.agent == nil {
		return fmt.Errorf("agent is unavailable")
	}
	return s.agent.FileDelete(ctx, absolutePath)
}

func (s *FileService) Rename(ctx context.Context, siteID, oldPath, newPath string) error {
	if err := s.ensureAgent(); err != nil {
		return err
	}
	absoluteOld, err := s.resolvePath(ctx, siteID, oldPath)
	if err != nil {
		return err
	}
	absoluteNew, err := s.resolvePath(ctx, siteID, newPath)
	if err != nil {
		return err
	}
	return s.agent.FileRename(ctx, absoluteOld, absoluteNew)
}

func (s *FileService) Copy(ctx context.Context, siteID, sourcePath, destinationPath string) error {
	if err := s.ensureAgent(); err != nil {
		return err
	}
	source, err := s.resolvePath(ctx, siteID, sourcePath)
	if err != nil {
		return err
	}
	destination, err := s.resolvePath(ctx, siteID, destinationPath)
	if err != nil {
		return err
	}
	return s.agent.FileCopy(ctx, source, destination)
}

func (s *FileService) Chmod(ctx context.Context, siteID, relativePath string, mode uint32) error {
	if err := s.ensureAgent(); err != nil {
		return err
	}
	absolutePath, err := s.resolvePath(ctx, siteID, relativePath)
	if err != nil {
		return err
	}
	return s.agent.FileChmod(ctx, absolutePath, mode)
}

func (s *FileService) Chown(ctx context.Context, siteID, relativePath string, uid, gid int) error {
	if err := s.ensureAgent(); err != nil {
		return err
	}
	absolutePath, err := s.resolvePath(ctx, siteID, relativePath)
	if err != nil {
		return err
	}
	return s.agent.FileChown(ctx, absolutePath, uid, gid)
}

func (s *FileService) CreateArchive(ctx context.Context, siteID string, relativePaths []string, destinationPath string) error {
	if err := s.ensureAgent(); err != nil {
		return err
	}
	if len(relativePaths) == 0 {
		return apperrors.InvalidInput("at least one archive source is required")
	}
	sources := make([]string, 0, len(relativePaths))
	for _, relativePath := range relativePaths {
		absolutePath, err := s.resolvePath(ctx, siteID, relativePath)
		if err != nil {
			return err
		}
		sources = append(sources, absolutePath)
	}
	destination, err := s.resolvePath(ctx, siteID, destinationPath)
	if err != nil {
		return err
	}
	return s.agent.ArchiveCreate(ctx, sources, destination)
}

func (s *FileService) ExtractArchive(ctx context.Context, siteID, sourcePath, destinationPath string) error {
	if err := s.ensureAgent(); err != nil {
		return err
	}
	source, err := s.resolvePath(ctx, siteID, sourcePath)
	if err != nil {
		return err
	}
	destination, err := s.resolvePath(ctx, siteID, destinationPath)
	if err != nil {
		return err
	}
	return s.agent.ArchiveExtract(ctx, source, destination)
}
