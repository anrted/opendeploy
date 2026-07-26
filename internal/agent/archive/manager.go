package archive

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Create creates an archive of the specified type containing the given files.
// format can be: zip, tar, tar.gz, tar.xz, tar.bz2, 7z
func Create(ctx context.Context, format, dest string, files []string) error {
	var cmd *exec.Cmd
	switch format {
	case "zip":
		args := append([]string{"-r", "-q", dest}, files...)
		cmd = exec.CommandContext(ctx, "zip", args...)
	case "tar":
		args := append([]string{"-cf", dest}, files...)
		cmd = exec.CommandContext(ctx, "tar", args...)
	case "tar.gz":
		args := append([]string{"-czf", dest}, files...)
		cmd = exec.CommandContext(ctx, "tar", args...)
	case "tar.xz":
		args := append([]string{"-cJf", dest}, files...)
		cmd = exec.CommandContext(ctx, "tar", args...)
	case "tar.bz2":
		args := append([]string{"-cjf", dest}, files...)
		cmd = exec.CommandContext(ctx, "tar", args...)
	case "7z":
		args := append([]string{"a", dest}, files...)
		cmd = exec.CommandContext(ctx, "7z", args...)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("archive create failed: %w, output: %s", err, string(out))
	}
	return nil
}

// Extract extracts an archive to the specified directory.
func Extract(ctx context.Context, archive, dest string) error {
	ext := strings.ToLower(filepath.Ext(archive))
	if strings.HasSuffix(strings.ToLower(archive), ".tar.gz") || strings.HasSuffix(strings.ToLower(archive), ".tgz") {
		ext = ".tar.gz"
	} else if strings.HasSuffix(strings.ToLower(archive), ".tar.xz") || strings.HasSuffix(strings.ToLower(archive), ".txz") {
		ext = ".tar.xz"
	} else if strings.HasSuffix(strings.ToLower(archive), ".tar.bz2") || strings.HasSuffix(strings.ToLower(archive), ".tbz2") {
		ext = ".tar.bz2"
	}

	var cmd *exec.Cmd
	switch ext {
	case ".zip":
		cmd = exec.CommandContext(ctx, "unzip", "-q", "-o", archive, "-d", dest)
	case ".tar":
		cmd = exec.CommandContext(ctx, "tar", "-xf", archive, "-C", dest)
	case ".tar.gz", ".tgz":
		cmd = exec.CommandContext(ctx, "tar", "-xzf", archive, "-C", dest)
	case ".tar.xz", ".txz":
		cmd = exec.CommandContext(ctx, "tar", "-xJf", archive, "-C", dest)
	case ".tar.bz2", ".tbz2":
		cmd = exec.CommandContext(ctx, "tar", "-xjf", archive, "-C", dest)
	case ".7z":
		cmd = exec.CommandContext(ctx, "7z", "x", archive, "-o"+dest, "-y")
	default:
		return fmt.Errorf("unsupported format for extraction: %s", ext)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("archive extract failed: %w, output: %s", err, string(out))
	}
	return nil
}
