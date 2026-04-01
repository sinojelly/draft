package yike

import (
	"encoding/json"
	"testing"
)

func TestExtractYikeString(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{"hello", "hello"},
		{float64(123), "123"},
		{json.Number("456"), "456"},
		{nil, ""},
		{123, "123"},
	}

	for _, tt := range tests {
		got := extractYikeString(tt.input)
		if got != tt.expected {
			t.Errorf("extractYikeString(%v) = %s; want %s", tt.input, got, tt.expected)
		}
	}
}

func TestExtractIDFromMap(t *testing.T) {
	data := map[string]interface{}{
		"errno": 0,
		"data": map[string]interface{}{
			"fs_id": json.Number("1234567890123456789"),
		},
		"info": map[string]interface{}{
			"album_id": "987654321",
		},
	}

	fsID := extractIDFromMap(data, "fs_id")
	if fsID != 1234567890123456789 {
		t.Errorf("Expected fs_id 1234567890123456789, got %d", fsID)
	}

	albumID := extractIDFromMap(data, "album_id")
	if albumID != 987654321 {
		t.Errorf("Expected album_id 987654321, got %d", albumID)
	}
}

func TestCalculateAlbumName(t *testing.T) {
	plugin := &YikePlugin{}

	tests := []struct {
		remotePath string
		expected   string
	}{
		{"/youa/web/test6/image.jpg", "test6"},
		{"/youa/web/2024/Trip/pic.jpg", "2024_Trip"},
		{"/youa/web/Vacation/old.png", "Vacation"},
		{"/youa/web/image.jpg", "SyncGhost_Default"},
		{"/youa/web/apps/SyncGhost/MyAlbum/file.jpg", "MyAlbum"},
		{"/youa/web/syncghost/test.jpg", "SyncGhost_Default"},
		{"/youa/web/A/B/C/file.jpg", "A_B_C"},
		{"image.jpg", "SyncGhost_Default"},
	}

	for _, tt := range tests {
		got := plugin.calculateAlbumName(tt.remotePath)
		if got != tt.expected {
			t.Errorf("calculateAlbumName(%s) = %s; want %s", tt.remotePath, got, tt.expected)
		}
	}
}

func TestGetAlbumIDByPath_Cache(t *testing.T) {
	plugin := NewYikePlugin("bduss", "stoken")
	
	// Pre-fill cache
	plugin.mu.Lock()
	plugin.albumCache["TestAlbum"] = "12345|TID999"
	plugin.mu.Unlock()

	// Test cache hit
	id, tid, err := plugin.GetAlbumIDByPath("local", "/youa/web/TestAlbum/file.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if id != "12345" || tid != "TID999" {
		t.Errorf("Cache hit failed: got id=%s, tid=%s; want 12345, TID999", id, tid)
	}
}
