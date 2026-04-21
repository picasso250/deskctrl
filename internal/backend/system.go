package backend

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type System struct {
	pwshPath   string
	scriptsDir string
}

func NewSystem(pwshPath string) (*System, error) {
	scriptsDir, err := resolveScriptsDir()
	if err != nil {
		return nil, err
	}

	return &System{
		pwshPath:   pwshPath,
		scriptsDir: scriptsDir,
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
