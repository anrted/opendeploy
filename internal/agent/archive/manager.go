package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	maxArchiveEntries = 10_000
	maxExtractedBytes = int64(1 << 30)
)

// Create invokes a fixed archiver binary with structured arguments. Sources and
// destination must already be resolved through the Agent filesystem boundary.
func Create(ctx context.Context, format, dest string, files []string) error {
	if len(files) == 0 || len(files) > maxArchiveEntries {
		return fmt.Errorf("archive: invalid source count")
	}
	var command string
	var args []string
	switch format {
	case "zip":
		command, args = "zip", append([]string{"-r", "-q", dest}, files...)
	case "tar":
		command, args = "tar", append([]string{"-cf", dest, "--"}, files...)
	case "tar.gz":
		command, args = "tar", append([]string{"-czf", dest, "--"}, files...)
	case "tar.xz":
		command, args = "tar", append([]string{"-cJf", dest, "--"}, files...)
	case "tar.bz2":
		command, args = "tar", append([]string{"-cjf", dest, "--"}, files...)
	case "7z":
		command, args = "7z", append([]string{"a", "--", dest}, files...)
	default:
		return fmt.Errorf("archive: unsupported format %q", format)
	}
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}
	output, err := limitedCombinedOutput(cmd, 1<<20)
	if err != nil {
		return fmt.Errorf("archive: create failed: %w: %s", err, output)
	}
	return nil
}

// Extract performs entry-by-entry extraction in-process. Absolute paths,
// traversal, links, devices and archive bombs are rejected before they can
// escape the destination. Formats without a safe standard-library decoder are
// intentionally rejected rather than delegated to a permissive utility.
func Extract(ctx context.Context, source, destination string) error {
	if err := os.MkdirAll(destination, 0o750); err != nil {
		return fmt.Errorf("archive: create destination: %w", err)
	}
	lower := strings.ToLower(source)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZIP(ctx, source, destination)
	case strings.HasSuffix(lower, ".tar"):
		return extractTARFile(ctx, source, destination, false)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTARFile(ctx, source, destination, true)
	default:
		return fmt.Errorf("archive: secure extraction supports zip, tar and tar.gz only")
	}
}

func extractZIP(ctx context.Context, source, destination string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return fmt.Errorf("archive: open zip: %w", err)
	}
	defer reader.Close()
	if len(reader.File) > maxArchiveEntries {
		return fmt.Errorf("archive: too many entries")
	}
	var total int64
	for _, entry := range reader.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		target, err := safeTarget(destination, entry.Name)
		if err != nil {
			return err
		}
		mode := entry.Mode()
		if mode&os.ModeSymlink != 0 || mode&os.ModeType != 0 && !mode.IsDir() {
			return fmt.Errorf("archive: unsupported zip entry type %q", entry.Name)
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("archive: create directory: %w", err)
			}
			continue
		}
		total += int64(entry.UncompressedSize64)
		if total > maxExtractedBytes {
			return fmt.Errorf("archive: extracted size limit exceeded")
		}
		if err := writeZIPEntry(entry, target); err != nil {
			return err
		}
	}
	return nil
}

func writeZIPEntry(entry *zip.File, target string) error {
	if err := ensureSafeParent(target); err != nil {
		return err
	}
	source, err := entry.Open()
	if err != nil {
		return fmt.Errorf("archive: open entry: %w", err)
	}
	defer source.Close()
	mode := entry.Mode().Perm()
	if mode == 0 {
		mode = 0o640
	}
	mode &^= 0o022
	destination, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("archive: create entry: %w", err)
	}
	_, copyErr := io.Copy(destination, io.LimitReader(source, maxExtractedBytes+1))
	closeErr := destination.Close()
	if copyErr != nil {
		return fmt.Errorf("archive: write entry: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("archive: close entry: %w", closeErr)
	}
	return nil
}

func extractTARFile(ctx context.Context, source, destination string, compressed bool) error {
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("archive: open tar: %w", err)
	}
	defer file.Close()
	var reader io.Reader = file
	if compressed {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return fmt.Errorf("archive: open gzip: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}
	return extractTAR(ctx, tar.NewReader(reader), destination)
}

func extractTAR(ctx context.Context, reader *tar.Reader, destination string) error {
	var entries int
	var total int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("archive: read tar: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if entries > maxArchiveEntries {
			return fmt.Errorf("archive: too many entries")
		}
		target, err := safeTarget(destination, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("archive: create directory: %w", err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || total+header.Size > maxExtractedBytes {
				return fmt.Errorf("archive: extracted size limit exceeded")
			}
			total += header.Size
			if err := writeTAREntry(reader, target, header.Size, os.FileMode(header.Mode)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("archive: links and special entries are forbidden: %q", header.Name)
		}
	}
}

func writeTAREntry(reader io.Reader, target string, size int64, mode os.FileMode) error {
	if err := ensureSafeParent(target); err != nil {
		return err
	}
	mode = mode.Perm() &^ 0o022
	if mode == 0 {
		mode = 0o640
	}
	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("archive: create entry: %w", err)
	}
	written, copyErr := io.CopyN(file, reader, size)
	closeErr := file.Close()
	if copyErr != nil || written != size {
		return fmt.Errorf("archive: truncated entry: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("archive: close entry: %w", closeErr)
	}
	return nil
}

func safeTarget(root, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) || strings.HasPrefix(filepath.ToSlash(name), "/") || strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("archive: invalid entry path")
	}

	target := filepath.Join(root, filepath.FromSlash(name))
	if !strings.HasPrefix(target, filepath.Clean(root)+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive: entry escapes destination: %q", name)
	}

	return target, nil
}

func ensureSafeParent(target string) error {
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return fmt.Errorf("archive: create parent: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fmt.Errorf("archive: resolve parent: %w", err)
	}
	if resolved != parent {
		return fmt.Errorf("archive: symlink parent is forbidden")
	}
	return nil
}

func limitedCombinedOutput(cmd *exec.Cmd, limit int64) (string, error) {
	var output limitedWriter
	output.remaining = limit
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	return output.String(), err
}

type limitedWriter struct {
	buffer    strings.Builder
	remaining int64
	truncated bool
}

func (w *limitedWriter) Write(value []byte) (int, error) {
	original := len(value)
	if int64(len(value)) > w.remaining {
		value = value[:max(0, int(w.remaining))]
		w.truncated = true
	}
	_, _ = w.buffer.Write(value)
	w.remaining -= int64(len(value))
	return original, nil
}

func (w *limitedWriter) String() string {
	if w.truncated {
		return w.buffer.String() + "\n[output truncated]"
	}
	return w.buffer.String()
}
