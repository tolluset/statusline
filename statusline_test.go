package main

import (
	"bytes"
	"encoding/json"
	"io"
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