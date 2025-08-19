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

	// Get git branch and status if in a git repository
	var gitBranch string
	var gitStatus string
	if isGitRepo(data.Workspace.CurrentDir) {
		gitBranch = getGitBranch(data.Workspace.CurrentDir)
		gitStatus = getGitStatus(data.Workspace.CurrentDir)
	}

	// Shorten the path display
	pwdShort := shortenPath(data.Workspace.CurrentDir, currentUser.HomeDir, data.Workspace.ProjectDir)

	if gitBranch != "" {
		if gitStatus != "" {
			template := `%s%s %s`
			output := fmt.Sprintf(template,
				fmt.Sprintf("\033[36m%s\033[0m", gitBranch),
				gitStatus,
				fmt.Sprintf("\033[35m%s\033[0m", pwdShort))
			fmt.Print(output)
		} else {
			template := `%s %s`
			output := fmt.Sprintf(template,
				fmt.Sprintf("\033[36m%s\033[0m", gitBranch),
				fmt.Sprintf("\033[35m%s\033[0m", pwdShort))
			fmt.Print(output)
		}
	} else {
		template := `%s`
		output := fmt.Sprintf(template,
			fmt.Sprintf("\033[35m%s\033[0m", pwdShort))
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

func getGitStatus(dir string) string {
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain=v1")
	cmd.Stderr = nil
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return ""
	}

	var statusParts []string
	
	// Staged changes
	stagedAdded := 0
	stagedModified := 0
	stagedDeleted := 0
	
	// Unstaged changes
	unstagedAdded := 0
	unstagedModified := 0
	unstagedDeleted := 0

	for _, line := range lines {
		if len(line) < 2 {
			continue
		}

		stagedStatus := line[0]
		workingStatus := line[1]

		// Count staged changes
		if stagedStatus != ' ' && stagedStatus != '?' {
			switch stagedStatus {
			case 'A':
				stagedAdded++
			case 'D':
				stagedDeleted++
			case 'M', 'R', 'C':
				stagedModified++
			}
		}

		// Count unstaged changes
		if workingStatus != ' ' && workingStatus != '?' {
			switch workingStatus {
			case 'M':
				unstagedModified++
			case 'D':
				unstagedDeleted++
			}
		}
		
		// Handle untracked files as unstaged additions
		if stagedStatus == '?' && workingStatus == '?' {
			unstagedAdded++
		}
	}

	// Show staged changes
	if stagedAdded > 0 || stagedModified > 0 || stagedDeleted > 0 {
		var parts []string
		if stagedAdded > 0 {
			parts = append(parts, fmt.Sprintf("\033[32m+%d\033[0m", stagedAdded))
		}
		if stagedModified > 0 {
			parts = append(parts, fmt.Sprintf("\033[33m~%d\033[0m", stagedModified))
		}
		if stagedDeleted > 0 {
			parts = append(parts, fmt.Sprintf("\033[31m-%d\033[0m", stagedDeleted))
		}
		if len(parts) > 0 {
			statusParts = append(statusParts, strings.Join(parts, ""))
		}
	}

	// Show unstaged changes
	if unstagedAdded > 0 || unstagedModified > 0 || unstagedDeleted > 0 {
		var parts []string
		if unstagedAdded > 0 {
			parts = append(parts, fmt.Sprintf("\033[92m+%d\033[0m", unstagedAdded))
		}
		if unstagedModified > 0 {
			parts = append(parts, fmt.Sprintf("\033[93m~%d\033[0m", unstagedModified))
		}
		if unstagedDeleted > 0 {
			parts = append(parts, fmt.Sprintf("\033[91m-%d\033[0m", unstagedDeleted))
		}
		if len(parts) > 0 {
			statusParts = append(statusParts, strings.Join(parts, ""))
		}
	}

	if len(statusParts) > 0 {
		return " " + strings.Join(statusParts, " ")
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
