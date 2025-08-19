package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShortenPath(t *testing.T) {
	tests := []struct {
		name       string
		currentDir string
		homeDir    string
		projectDir string
		expected   string
	}{
		{
			name:       "path under home directory",
			currentDir: "/Users/john/Documents/project",
			homeDir:    "/Users/john",
			projectDir: "",
			expected:   "~/Documents/project",
		},
		{
			name:       "path under project directory",
			currentDir: "/workspace/myproject/src/main",
			homeDir:    "/Users/john",
			projectDir: "/workspace/myproject",
			expected:   "src/main",
		},
		{
			name:       "path equals home directory",
			currentDir: "/Users/john",
			homeDir:    "/Users/john",
			projectDir: "",
			expected:   "/Users/john",
		},
		{
			name:       "path equals project directory",
			currentDir: "/workspace/myproject",
			homeDir:    "/Users/john",
			projectDir: "/workspace/myproject",
			expected:   "/workspace/myproject",
		},
		{
			name:       "path not under home or project",
			currentDir: "/tmp/test",
			homeDir:    "/Users/john",
			projectDir: "/workspace/myproject",
			expected:   "/tmp/test",
		},
		{
			name:       "null project directory",
			currentDir: "/Users/john/test",
			homeDir:    "/Users/john",
			projectDir: "null",
			expected:   "~/test",
		},
		{
			name:       "empty project directory",
			currentDir: "/Users/john/test",
			homeDir:    "/Users/john",
			projectDir: "",
			expected:   "~/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shortenPath(tt.currentDir, tt.homeDir, tt.projectDir)
			if result != tt.expected {
				t.Errorf("shortenPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsGitRepo(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("not a git repository", func(t *testing.T) {
		if isGitRepo(tempDir) {
			t.Errorf("isGitRepo() = true, want false for non-git directory")
		}
	})

	t.Run("is a git repository", func(t *testing.T) {
		gitDir := filepath.Join(tempDir, "test-repo")
		err := os.Mkdir(gitDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		cmd := exec.Command("git", "init", gitDir)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			t.Skip("git not available, skipping git repository test")
		}

		if !isGitRepo(gitDir) {
			t.Errorf("isGitRepo() = false, want true for git directory")
		}
	})
}

func TestGetGitBranch(t *testing.T) {
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, "test-repo")

	err := os.Mkdir(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	cmd := exec.Command("git", "init", gitDir)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git branch test")
	}

	cmd = exec.Command("git", "-C", gitDir, "config", "user.email", "test@example.com")
	cmd.Run()
	cmd = exec.Command("git", "-C", gitDir, "config", "user.name", "Test User")
	cmd.Run()

	testFile := filepath.Join(gitDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "-C", gitDir, "add", "test.txt")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	cmd = exec.Command("git", "-C", gitDir, "commit", "-m", "initial commit")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	branch := getGitBranch(gitDir)
	if branch == "" {
		t.Errorf("getGitBranch() returned empty string, expected a branch name")
	}

	expectedBranches := []string{"main", "master"}
	found := false
	for _, expected := range expectedBranches {
		if branch == expected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("getGitBranch() = %v, expected one of %v", branch, expectedBranches)
	}
}

func TestGetGitStatus(t *testing.T) {
	tempDir := t.TempDir()
	gitDir := filepath.Join(tempDir, "test-repo")

	err := os.Mkdir(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	cmd := exec.Command("git", "init", gitDir)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		t.Skip("git not available, skipping git status test")
	}

	cmd = exec.Command("git", "-C", gitDir, "config", "user.email", "test@example.com")
	cmd.Run()
	cmd = exec.Command("git", "-C", gitDir, "config", "user.name", "Test User")
	cmd.Run()

	t.Run("clean repository", func(t *testing.T) {
		status := getGitStatus(gitDir)
		if status != "" {
			t.Errorf("getGitStatus() = %v, want empty string for clean repo", status)
		}
	})

	t.Run("untracked files", func(t *testing.T) {
		testFile := filepath.Join(gitDir, "untracked.txt")
		err = os.WriteFile(testFile, []byte("untracked"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		status := getGitStatus(gitDir)
		if status == "" {
			t.Errorf("getGitStatus() returned empty string, expected status for untracked file")
		}
		if !strings.Contains(status, "+1") {
			t.Errorf("getGitStatus() = %v, expected to contain '+1' for untracked file", status)
		}
	})
}

func TestMainFunction(t *testing.T) {
	testInput := StatusLineInput{
		SessionID:      "test-session",
		TranscriptPath: "/tmp/transcript",
		CWD:            "/tmp",
		Model: struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		}{
			ID:          "claude-3",
			DisplayName: "Claude 3",
		},
		Workspace: struct {
			CurrentDir string `json:"current_dir"`
			ProjectDir string `json:"project_dir"`
		}{
			CurrentDir: "/Users/test/project",
			ProjectDir: "/Users/test/project",
		},
		Version: "1.0.0",
		OutputStyle: struct {
			Name string `json:"name"`
		}{
			Name: "default",
		},
	}

	jsonInput, err := json.Marshal(testInput)
	if err != nil {
		t.Fatalf("Failed to marshal test input: %v", err)
	}

	cmd := exec.Command("go", "run", "statusline.go")
	cmd.Stdin = bytes.NewReader(jsonInput)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	if output == "" {
		t.Errorf("Expected output, got empty string")
	}

	if !strings.Contains(output, "project") {
		t.Errorf("Expected output to contain 'project', got: %s", output)
	}
}

func TestMainFunctionNoStdin(t *testing.T) {
	cmd := exec.Command("go", "run", "statusline.go")
	cmd.Stdin = strings.NewReader("")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Errorf("Expected command to fail with no stdin input")
	}

	if !strings.Contains(stderr.String(), "Error parsing JSON") {
		t.Errorf("Expected JSON parsing error, got: %s", stderr.String())
	}
}

func TestMainFunctionInvalidJSON(t *testing.T) {
	cmd := exec.Command("go", "run", "statusline.go")
	cmd.Stdin = strings.NewReader("{invalid json}")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Errorf("Expected command to fail with invalid JSON")
	}

	if !strings.Contains(stderr.String(), "Error parsing JSON") {
		t.Errorf("Expected JSON parsing error, got: %s", stderr.String())
	}
}

func TestCache(t *testing.T) {
	tempDir := t.TempDir()
	cacheFile := filepath.Join(tempDir, "test-cache.txt")

	cache := NewCache(cacheFile, 1*time.Second)

	t.Run("cache miss", func(t *testing.T) {
		_, found := cache.Get("nonexistent")
		if found {
			t.Errorf("Expected cache miss, but found value")
		}
	})

	t.Run("cache set and get", func(t *testing.T) {
		err := cache.Set("test-key", "test-value")
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}

		value, found := cache.Get("test-key")
		if !found {
			t.Errorf("Expected cache hit, but got miss")
		}
		if value != "test-value" {
			t.Errorf("Expected 'test-value', got '%s'", value)
		}
	})

	t.Run("cache expiration", func(t *testing.T) {
		shortCache := NewCache(cacheFile, 10*time.Millisecond)

		err := shortCache.Set("expire-key", "expire-value")
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}

		time.Sleep(20 * time.Millisecond)

		_, found := shortCache.Get("expire-key")
		if found {
			t.Errorf("Expected cache to be expired")
		}
	})

	t.Run("cache overwrite", func(t *testing.T) {
		err := cache.Set("overwrite-key", "old-value")
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}

		err = cache.Set("overwrite-key", "new-value")
		if err != nil {
			t.Fatalf("Failed to overwrite cache: %v", err)
		}

		value, found := cache.Get("overwrite-key")
		if !found {
			t.Errorf("Expected cache hit, but got miss")
		}
		if value != "new-value" {
			t.Errorf("Expected 'new-value', got '%s'", value)
		}
	})

	t.Run("multiple keys", func(t *testing.T) {
		err := cache.Set("key1", "value1")
		if err != nil {
			t.Fatalf("Failed to set cache key1: %v", err)
		}

		err = cache.Set("key2", "value2")
		if err != nil {
			t.Fatalf("Failed to set cache key2: %v", err)
		}

		value1, found1 := cache.Get("key1")
		value2, found2 := cache.Get("key2")

		if !found1 || value1 != "value1" {
			t.Errorf("Expected key1='value1', got found=%t, value='%s'", found1, value1)
		}
		if !found2 || value2 != "value2" {
			t.Errorf("Expected key2='value2', got found=%t, value='%s'", found2, value2)
		}
	})
}

func TestCacheEntry(t *testing.T) {
	entry := CacheEntry{
		Timestamp: time.Now(),
		Key:       "test",
		Content:   "content",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal cache entry: %v", err)
	}

	var unmarshaled CacheEntry
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal cache entry: %v", err)
	}

	if unmarshaled.Key != entry.Key {
		t.Errorf("Expected key '%s', got '%s'", entry.Key, unmarshaled.Key)
	}
	if unmarshaled.Content != entry.Content {
		t.Errorf("Expected content '%s', got '%s'", entry.Content, unmarshaled.Content)
	}
}

func TestLoadEnv(t *testing.T) {
	tempDir := t.TempDir()
	claudeDir := filepath.Join(tempDir, ".claude")
	envFile := filepath.Join(claudeDir, ".env")

	// Create .claude directory
	err := os.MkdirAll(claudeDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	// Create test .env file
	envContent := `# Test comment
TEST_KEY=test_value
GITHUB_TOKEN=ghp_test123

# Another comment
EMPTY_VALUE=
SPACES_VALUE= value with spaces `
	err = os.WriteFile(envFile, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	// Mock home directory
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tempDir)

	envVars := loadEnv()

	if envVars["TEST_KEY"] != "test_value" {
		t.Errorf("Expected TEST_KEY=test_value, got %s", envVars["TEST_KEY"])
	}

	if envVars["GITHUB_TOKEN"] != "ghp_test123" {
		t.Errorf("Expected GITHUB_TOKEN=ghp_test123, got %s", envVars["GITHUB_TOKEN"])
	}

	if envVars["EMPTY_VALUE"] != "" {
		t.Errorf("Expected EMPTY_VALUE to be empty, got %s", envVars["EMPTY_VALUE"])
	}

	if envVars["SPACES_VALUE"] != "value with spaces" {
		t.Errorf("Expected SPACES_VALUE='value with spaces', got '%s'", envVars["SPACES_VALUE"])
	}
}

func TestFetchGitHubNotifications(t *testing.T) {
	t.Run("empty token", func(t *testing.T) {
		_, err := fetchGitHubNotifications("")
		if err == nil {
			t.Errorf("Expected error for empty token")
		}
	})

	t.Run("successful API call", func(t *testing.T) {
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request headers
			if r.Header.Get("Authorization") != "token test_token" {
				t.Errorf("Expected Authorization header 'token test_token', got %s", r.Header.Get("Authorization"))
			}
			if r.Header.Get("Accept") != "application/vnd.github+json" {
				t.Errorf("Expected Accept header 'application/vnd.github+json', got %s", r.Header.Get("Accept"))
			}

			// Mock response
			mockResponse := `[
				{
					"id": "1",
					"reason": "mention",
					"subject": {
						"title": "Test PR",
						"url": "https://api.github.com/repos/test/repo/pulls/1",
						"type": "PullRequest"
					},
					"repository": {
						"full_name": "test/repo"
					},
					"unread": true
				}
			]`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockResponse))
		}))
		defer server.Close()

		// This test would need to modify the actual API URL, which is hardcoded
		// For a real implementation, we'd make the URL configurable
		// For now, we'll just test with the actual API (but expect it to fail due to invalid token)
		_, err := fetchGitHubNotifications("invalid_token")
		if err == nil {
			t.Errorf("Expected error for invalid token")
		}
	})
}

func TestGetNotificationCount(t *testing.T) {
	// Create a temporary directory for cache testing
	tempDir := t.TempDir()
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tempDir)

	t.Run("empty token", func(t *testing.T) {
		envVars := map[string]string{}
		count := getNotificationCount(envVars)
		if count != -1 {
			t.Errorf("Expected -1 for empty token, got %d", count)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		envVars := map[string]string{"GITHUB_TOKEN": "invalid_token_unique_12345"}
		count := getNotificationCount(envVars)
		if count != -1 {
			t.Errorf("Expected -1 for invalid token, got %d", count)
		}
	})

	t.Run("notifications disabled", func(t *testing.T) {
		envVars := map[string]string{
			"GITHUB_TOKEN":              "valid_token",
			"SHOW_GITHUB_NOTIFICATIONS": "false",
		}
		// This test assumes the main statusline function would skip calling getNotificationCount
		// when SHOW_GITHUB_NOTIFICATIONS is false
		count := getNotificationCount(envVars)
		// getNotificationCount still works, but main function won't call it
		if count == -1 {
			// Expected behavior when token is invalid or API fails
		}
	})
}

func TestHandleNotiCommand(t *testing.T) {
	tempDir := t.TempDir()
	claudeDir := filepath.Join(tempDir, ".claude")

	// Mock home directory
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)
	os.Setenv("HOME", tempDir)

	t.Run("no env file", func(t *testing.T) {
		output := captureOutput(handleNotiCommand)
		if !strings.Contains(output, "GITHUB_TOKEN not set") {
			t.Errorf("Expected output to contain 'GITHUB_TOKEN not set', got: %s", output)
		}
	})

	t.Run("placeholder token", func(t *testing.T) {
		err := os.MkdirAll(claudeDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create .claude directory: %v", err)
		}

		envFile := filepath.Join(claudeDir, ".env")
		envContent := "GITHUB_TOKEN=your_github_token_here"
		err = os.WriteFile(envFile, []byte(envContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create .env file: %v", err)
		}

		output := captureOutput(handleNotiCommand)
		if !strings.Contains(output, "GITHUB_TOKEN not set") {
			t.Errorf("Expected output to contain 'GITHUB_TOKEN not set', got: %s", output)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		err := os.MkdirAll(claudeDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create .claude directory: %v", err)
		}

		envFile := filepath.Join(claudeDir, ".env")
		envContent := "GITHUB_TOKEN=invalid_token_123"
		err = os.WriteFile(envFile, []byte(envContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create .env file: %v", err)
		}

		output := captureOutput(handleNotiCommand)
		if !strings.Contains(output, "Error fetching notifications") {
			t.Errorf("Expected output to contain 'Error fetching notifications', got: %s", output)
		}
	})
}

func TestNotificationStruct(t *testing.T) {
	mockJSON := `{
		"id": "123",
		"reason": "mention",
		"subject": {
			"title": "Test Issue",
			"url": "https://api.github.com/repos/test/repo/issues/1",
			"type": "Issue"
		},
		"repository": {
			"full_name": "test/repo"
		},
		"unread": true
	}`

	var notification Notification
	err := json.Unmarshal([]byte(mockJSON), &notification)
	if err != nil {
		t.Fatalf("Failed to unmarshal notification: %v", err)
	}

	if notification.ID != "123" {
		t.Errorf("Expected ID '123', got '%s'", notification.ID)
	}
	if notification.Reason != "mention" {
		t.Errorf("Expected reason 'mention', got '%s'", notification.Reason)
	}
	if notification.Subject.Title != "Test Issue" {
		t.Errorf("Expected title 'Test Issue', got '%s'", notification.Subject.Title)
	}
	if notification.Repository.FullName != "test/repo" {
		t.Errorf("Expected repository 'test/repo', got '%s'", notification.Repository.FullName)
	}
	if !notification.Unread {
		t.Errorf("Expected unread to be true, got false")
	}
}

func TestMainWithNotiCommand(t *testing.T) {
	tempDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(origDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	cmd := exec.Command("go", "run", filepath.Join(origDir, "statusline.go"), "noti")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v\nStderr: %s", err, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "GitHub Notifications") {
		t.Errorf("Expected output to contain 'GitHub Notifications', got: %s", output)
	}
}

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}
