package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCache_SetAndGet(t *testing.T) {
	tmpFile := createTempFile(t)
	defer os.Remove(tmpFile)
	
	cache := NewCache(tmpFile, 5*time.Minute)
	
	err := cache.Set("test_key", "test_content")
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}
	
	content, found := cache.Get("test_key")
	if !found {
		t.Fatal("Cache entry not found")
	}
	
	if content != "test_content" {
		t.Errorf("Expected 'test_content', got '%s'", content)
	}
}

func TestCache_GetNonExistentKey(t *testing.T) {
	tmpFile := createTempFile(t)
	defer os.Remove(tmpFile)
	
	cache := NewCache(tmpFile, 5*time.Minute)
	
	_, found := cache.Get("non_existent_key")
	if found {
		t.Error("Expected cache miss for non-existent key")
	}
}

func TestCache_TTLExpiration(t *testing.T) {
	tmpFile := createTempFile(t)
	defer os.Remove(tmpFile)
	
	cache := NewCache(tmpFile, 10*time.Millisecond)
	
	err := cache.Set("test_key", "test_content")
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}
	
	time.Sleep(20 * time.Millisecond)
	
	_, found := cache.Get("test_key")
	if found {
		t.Error("Expected cache miss due to TTL expiration")
	}
}

func TestCache_LatestEntryOverwrite(t *testing.T) {
	tmpFile := createTempFile(t)
	defer os.Remove(tmpFile)
	
	cache := NewCache(tmpFile, 5*time.Minute)
	
	err := cache.Set("test_key", "old_content")
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}
	
	err = cache.Set("test_key", "new_content")
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}
	
	content, found := cache.Get("test_key")
	if !found {
		t.Fatal("Cache entry not found")
	}
	
	if content != "new_content" {
		t.Errorf("Expected 'new_content', got '%s'", content)
	}
}

func TestCache_MultipleKeys(t *testing.T) {
	tmpFile := createTempFile(t)
	defer os.Remove(tmpFile)
	
	cache := NewCache(tmpFile, 5*time.Minute)
	
	err := cache.Set("key1", "content1")
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}
	
	err = cache.Set("key2", "content2")
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}
	
	content1, found1 := cache.Get("key1")
	if !found1 || content1 != "content1" {
		t.Errorf("Expected 'content1', got '%s', found: %v", content1, found1)
	}
	
	content2, found2 := cache.Get("key2")
	if !found2 || content2 != "content2" {
		t.Errorf("Expected 'content2', got '%s', found: %v", content2, found2)
	}
}

func TestCache_NonExistentFile(t *testing.T) {
	nonExistentFile := "/tmp/non_existent_cache_file"
	
	cache := NewCache(nonExistentFile, 5*time.Minute)
	
	_, found := cache.Get("test_key")
	if found {
		t.Error("Expected cache miss for non-existent file")
	}
	
	err := cache.Set("test_key", "test_content")
	if err != nil {
		t.Fatalf("Failed to create cache file: %v", err)
	}
	
	defer os.Remove(nonExistentFile)
	
	content, found := cache.Get("test_key")
	if !found {
		t.Fatal("Cache entry not found after creating file")
	}
	
	if content != "test_content" {
		t.Errorf("Expected 'test_content', got '%s'", content)
	}
}

func createTempFile(t *testing.T) string {
	tmpFile := filepath.Join(os.TempDir(), "cache_test_"+t.Name())
	return tmpFile
}