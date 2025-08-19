package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"time"
)

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