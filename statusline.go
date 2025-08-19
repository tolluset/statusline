package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

type StatusLineInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	Model          struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"model"`
	Workspace struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
	Version     string `json:"version"`
	OutputStyle struct {
		Name string `json:"name"`
	} `json:"output_style"`
}

func main() {
	// Read JSON input from stdin
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
		os.Exit(1)
	}

	var data StatusLineInput
	if err := json.Unmarshal(input, &data); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	// Get current user and hostname
	currentUser, err := user.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current user: %v\n", err)
		os.Exit(1)
	}

	// Get git branch if in a git repository
	var gitBranch string
	if isGitRepo(data.Workspace.CurrentDir) {
		gitBranch = getGitBranch(data.Workspace.CurrentDir)
	}

	// Shorten the path display
	pwdShort := shortenPath(data.Workspace.CurrentDir, currentUser.HomeDir, data.Workspace.ProjectDir)

	template := `%s %s [%s]`

	if gitBranch != "" {
		output := fmt.Sprintf(template,
			fmt.Sprintf("\033[36m%s\033[0m", gitBranch),
			fmt.Sprintf("\033[35m%s\033[0m", pwdShort),
			fmt.Sprintf("\033[2m%s\033[0m", data.Model.DisplayName))
		fmt.Print(output)
	} else {
		template := `%s [%s]`
		output := fmt.Sprintf(template,
			fmt.Sprintf("\033[35m%s\033[0m", pwdShort),
			fmt.Sprintf("\033[2m%s\033[0m", data.Model.DisplayName))
		fmt.Print(output)
	}
}

func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func getGitBranch(dir string) string {
	// Try to get symbolic ref (branch name)
	cmd := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD")
	cmd.Stderr = nil
	if output, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(output))
	}

	// If not on a branch, get short commit hash
	cmd = exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD")
	cmd.Stderr = nil
	if output, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(output))
	}

	return ""
}

func shortenPath(currentDir, homeDir, projectDir string) string {
	pwdShort := currentDir

	// Replace home directory with ~
	if currentDir != homeDir && strings.HasPrefix(currentDir, homeDir) {
		pwdShort = strings.Replace(currentDir, homeDir, "~", 1)
	}

	// If we're in a project and not at the project root, show relative path
	if currentDir != projectDir && projectDir != "null" && projectDir != "" {
		if strings.HasPrefix(currentDir, projectDir+"/") {
			pwdShort = strings.TrimPrefix(currentDir, projectDir+"/")
		}
	}

	return pwdShort
}
