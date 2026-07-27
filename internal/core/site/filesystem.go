package site

import (
	"context"

	"github.com/anrted/opendeploy/pkg/contract"
)

func (s *Service) resolveFilePath(ctx context.Context, siteID, relativePath string) (string, error) {
	return s.files.resolvePath(ctx, siteID, relativePath)
}

func (s *Service) ListFiles(ctx context.Context, siteID, relativePath string) ([]contract.FileInfo, error) {
	return s.files.List(ctx, siteID, relativePath)
}

func (s *Service) ReadFile(ctx context.Context, siteID, relativePath string) ([]byte, error) {
	return s.files.Read(ctx, siteID, relativePath)
}

func (s *Service) WriteFile(ctx context.Context, siteID, relativePath string, content []byte) error {
	return s.files.Write(ctx, siteID, relativePath, content)
}

func (s *Service) CreateDirectory(ctx context.Context, siteID, relativePath string) error {
	return s.files.CreateDirectory(ctx, siteID, relativePath)
}

func (s *Service) DeleteFile(ctx context.Context, siteID, relativePath string) error {
	return s.files.Delete(ctx, siteID, relativePath)
}

func (s *Service) RenameFile(ctx context.Context, siteID, oldPath, newPath string) error {
	return s.files.Rename(ctx, siteID, oldPath, newPath)
}

func (s *Service) CopyFile(ctx context.Context, siteID, sourcePath, destinationPath string) error {
	return s.files.Copy(ctx, siteID, sourcePath, destinationPath)
}

func (s *Service) ChmodFile(ctx context.Context, siteID, relativePath string, mode uint32) error {
	return s.files.Chmod(ctx, siteID, relativePath, mode)
}

func (s *Service) ChownFile(ctx context.Context, siteID, relativePath string, uid, gid int) error {
	return s.files.Chown(ctx, siteID, relativePath, uid, gid)
}

func (s *Service) CreateArchive(ctx context.Context, siteID string, relativePaths []string, destinationPath string) error {
	return s.files.CreateArchive(ctx, siteID, relativePaths, destinationPath)
}

func (s *Service) ExtractArchive(ctx context.Context, siteID, sourcePath, destinationPath string) error {
	return s.files.ExtractArchive(ctx, siteID, sourcePath, destinationPath)
}
