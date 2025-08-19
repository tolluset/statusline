package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"
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
	cmd := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD")
	cmd.Stderr = nil
	if output, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(output))
	}

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
	
	stagedAdded := 0
	stagedModified := 0
	stagedDeleted := 0
	
	unstagedAdded := 0
	unstagedModified := 0
	unstagedDeleted := 0

	for _, line := range lines {
		if len(line) < 2 {
			continue
		}

		stagedStatus := line[0]
		workingStatus := line[1]

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

		if workingStatus != ' ' && workingStatus != '?' {
			switch workingStatus {
			case 'M':
				unstagedModified++
			case 'D':
				unstagedDeleted++
			}
		}
		
		if stagedStatus == '?' && workingStatus == '?' {
			unstagedAdded++
		}
	}

	// Get staged changes statistics
	stagedStats := getGitDiffStat(dir, true)
	unstagedStats := getGitDiffStat(dir, false)

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
		statusText := strings.Join(parts, "")
		if stagedStats != "" {
			statusText += stagedStats
		}
		statusParts = append(statusParts, statusText)
	}

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
		statusText := strings.Join(parts, "")
		if unstagedStats != "" {
			statusText += unstagedStats
		}
		statusParts = append(statusParts, statusText)
	}

	if len(statusParts) > 0 {
		return " " + strings.Join(statusParts, " ")
	}
	return ""
}

func getGitDiffStat(dir string, staged bool) string {
	var cmd *exec.Cmd
	if staged {
		cmd = exec.Command("git", "-C", dir, "diff", "--cached", "--shortstat")
	} else {
		cmd = exec.Command("git", "-C", dir, "diff", "--shortstat")
	}
	cmd.Stderr = nil
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	statLine := strings.TrimSpace(string(output))
	if statLine == "" {
		return ""
	}

	// Parse shortstat output like "2 files changed, 150 insertions(+), 50 deletions(-)"
	var filesChanged, insertions, deletions int
	if strings.Contains(statLine, "file") {
		fmt.Sscanf(statLine, "%d file", &filesChanged)
	}
	if strings.Contains(statLine, "insertion") {
		parts := strings.Split(statLine, ", ")
		for _, part := range parts {
			if strings.Contains(part, "insertion") {
				fmt.Sscanf(part, "%d insertion", &insertions)
			}
		}
	}
	if strings.Contains(statLine, "deletion") {
		parts := strings.Split(statLine, ", ")
		for _, part := range parts {
			if strings.Contains(part, "deletion") {
				fmt.Sscanf(part, "%d deletion", &deletions)
			}
		}
	}

	var statParts []string
	if filesChanged > 0 {
		statParts = append(statParts, fmt.Sprintf("(\033[36m%df\033[0m", filesChanged))
	}
	if insertions > 0 {
		statParts = append(statParts, fmt.Sprintf("\033[32m+%d\033[0m", insertions))
	}
	if deletions > 0 {
		statParts = append(statParts, fmt.Sprintf("\033[31m-%d\033[0m", deletions))
	}

	if len(statParts) > 0 {
		result := strings.Join(statParts, "")
		if filesChanged > 0 {
			result += ")"
		}
		return result
	}
	return ""
}

func shortenPath(currentDir, homeDir, projectDir string) string {
	pwdShort := currentDir

	if currentDir != homeDir && strings.HasPrefix(currentDir, homeDir+"/") {
		pwdShort = "~" + strings.TrimPrefix(currentDir, homeDir)
	}

	if currentDir != projectDir && projectDir != "null" && projectDir != "" {
		if strings.HasPrefix(currentDir, projectDir+"/") {
			pwdShort = strings.TrimPrefix(currentDir, projectDir+"/")
		}
	}

	return pwdShort
}

type CacheEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Key       string    `json:"key"`
	Content   string    `json:"content"`
}

type Cache struct {
	FilePath string
	TTL      time.Duration
}

func NewCache(filePath string, ttl time.Duration) *Cache {
	return &Cache{
		FilePath: filePath,
		TTL:      ttl,
	}
}

func (c *Cache) Get(key string) (string, bool) {
	entry, found := c.getLatestEntry(key)
	if !found {
		return "", false
	}
	
	if c.isValid(entry) {
		return entry.Content, true
	}
	
	return "", false
}

func (c *Cache) Set(key, content string) error {
	entry := CacheEntry{
		Timestamp: time.Now(),
		Key:       key,
		Content:   content,
	}
	
	return c.appendEntry(entry)
}

func (c *Cache) getLatestEntry(key string) (CacheEntry, bool) {
	file, err := os.Open(c.FilePath)
	if err != nil {
		return CacheEntry{}, false
	}
	defer file.Close()
	
	var latestEntry CacheEntry
	found := false
	
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		var entry CacheEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		
		if entry.Key == key {
			latestEntry = entry
			found = true
		}
	}
	
	return latestEntry, found
}

func (c *Cache) appendEntry(entry CacheEntry) error {
	file, err := os.OpenFile(c.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	
	_, err = file.Write(append(data, '\n'))
	return err
}

func (c *Cache) isValid(entry CacheEntry) bool {
	return time.Since(entry.Timestamp) <= c.TTL
}

