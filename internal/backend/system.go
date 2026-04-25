package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type System struct {
	pwshPath   string
	scriptsDir string
	homeDir    string
}

type FileListing struct {
	Path    string      `json:"path"`
	Parent  string      `json:"parent,omitempty"`
	Entries []FileEntry `json:"entries"`
}

type FileEntry struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Type     string    `json:"type"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

func NewSystem(pwshPath string) (*System, error) {
	scriptsDir, err := resolveScriptsDir()
	if err != nil {
		return nil, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}

	return &System{
		pwshPath:   pwshPath,
		scriptsDir: scriptsDir,
		homeDir:    homeDir,
	}, nil
}

func (s *System) CaptureScreenshot(ctx context.Context) ([]byte, error) {
	tempFile, err := os.CreateTemp("", "deskctrl-*.png")
	if err != nil {
		return nil, fmt.Errorf("create temp screenshot file: %w", err)
	}

	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, fmt.Errorf("close temp screenshot file: %w", err)
	}
	defer os.Remove(tempPath)

	if _, err := s.runScript(ctx, "capture-screenshot.ps1", "-OutputPath", tempPath); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(tempPath)
	if err != nil {
		return nil, fmt.Errorf("read screenshot file: %w", err)
	}

	return data, nil
}

func (s *System) GetVolume(ctx context.Context) (int, error) {
	output, err := s.runScript(ctx, "volume.ps1", "-Action", "Get")
	if err != nil {
		return 0, err
	}

	level, err := parseLevel(output)
	if err != nil {
		return 0, err
	}

	return level, nil
}

func (s *System) SetVolume(ctx context.Context, level int) (int, error) {
	output, err := s.runScript(ctx, "volume.ps1", "-Action", "Set", "-Level", strconv.Itoa(clampLevel(level)))
	if err != nil {
		return 0, err
	}

	updatedLevel, err := parseLevel(output)
	if err != nil {
		return 0, err
	}

	return updatedLevel, nil
}

func (s *System) ListFiles(path string) (FileListing, error) {
	targetPath, err := s.resolveHomePath(path)
	if err != nil {
		return FileListing{}, err
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return FileListing{}, fmt.Errorf("read directory: %w", err)
	}

	listing := FileListing{
		Path:    targetPath,
		Entries: make([]FileEntry, 0, len(entries)),
	}

	if parent := filepath.Dir(targetPath); parent != targetPath {
		if s.isInsideHome(parent) {
			listing.Parent = parent
		}
	}

	for _, entry := range entries {
		if shouldHideFile(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		mode := info.Mode()
		if mode&os.ModeSymlink != 0 {
			continue
		}

		entryType := "file"
		size := info.Size()
		if entry.IsDir() {
			entryType = "dir"
			size = 0
		} else if !mode.IsRegular() {
			continue
		}

		listing.Entries = append(listing.Entries, FileEntry{
			Name:     entry.Name(),
			Path:     filepath.Join(targetPath, entry.Name()),
			Type:     entryType,
			Size:     size,
			Modified: info.ModTime(),
		})
	}

	sort.Slice(listing.Entries, func(i, j int) bool {
		left := listing.Entries[i]
		right := listing.Entries[j]
		if left.Type != right.Type {
			return left.Type == "dir"
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})

	return listing, nil
}

func (s *System) RunPrompt(ctx context.Context, runner string, prompt string) (string, error) {
	runner = strings.TrimSpace(strings.ToLower(runner))
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", fmt.Errorf("prompt is required")
	}

	var cmd *exec.Cmd
	switch runner {
	case "", "pi":
		cmd = exec.CommandContext(ctx, "pi", "-p", prompt)
		runner = "pi"
	case "codex":
		cmd = exec.CommandContext(ctx, "codex", prompt)
	default:
		return "", fmt.Errorf("unsupported runner %q", runner)
	}

	cmd.Dir = s.homeDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %s: %w: %s", runner, err, strings.TrimSpace(string(output)))
	}

	return strings.TrimSpace(string(output)), nil
}

func (s *System) resolveHomePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return filepath.Clean(s.homeDir), nil
	}

	targetPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	targetPath = filepath.Clean(targetPath)
	if !s.isInsideHome(targetPath) {
		return "", fmt.Errorf("path is outside home directory")
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return "", fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}

	return targetPath, nil
}

func (s *System) isInsideHome(path string) bool {
	rel, err := filepath.Rel(filepath.Clean(s.homeDir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func (s *System) runScript(ctx context.Context, scriptName string, args ...string) (string, error) {
	scriptPath := filepath.Join(s.scriptsDir, scriptName)

	commandArgs := []string{"-NoProfile", "-File", scriptPath}
	commandArgs = append(commandArgs, args...)

	cmd := exec.CommandContext(ctx, s.pwshPath, commandArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %s: %w: %s", scriptName, err, strings.TrimSpace(string(output)))
	}

	return strings.TrimSpace(string(output)), nil
}

func resolveScriptsDir() (string, error) {
	candidates := make([]string, 0, 4)

	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "scripts"))
	}

	if executable, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(exeDir, "scripts"),
			filepath.Join(exeDir, "..", "scripts"),
		)
	}

	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("cannot find scripts directory")
}

func parseLevel(raw string) (int, error) {
	level, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("parse volume level %q: %w", raw, err)
	}
	return clampLevel(level), nil
}

func clampLevel(level int) int {
	switch {
	case level < 0:
		return 0
	case level > 100:
		return 100
	default:
		return level
	}
}

func shouldHideFile(name string) bool {
	lowerName := strings.ToLower(name)
	if strings.HasPrefix(lowerName, ".") {
		return true
	}

	if lowerName == "appdata" || lowerName == "ntuser.ini" || strings.HasPrefix(lowerName, "ntuser.dat") {
		return true
	}

	switch lowerName {
	case "cloudflared-deskctrl.yml", ".git-credentials":
		return true
	default:
		return false
	}
}
