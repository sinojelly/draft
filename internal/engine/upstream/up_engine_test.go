package upstream

import (
	"reflect"
	"testing"
	"syncghost/internal/engine"
)

func TestDeduplicateBatch(t *testing.T) {
	e := &UpEngine{}

	tests := []struct {
		name     string
		batch    []engine.UpTaskEvent
		expected []engine.UpTaskEvent
	}{
		{
			name: "Remove redundant file delete",
			batch: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/A", IsDir: true},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/A/file1.txt", IsDir: false},
			},
			expected: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/A", IsDir: true},
			},
		},
		{
			name: "Remove redundant nested dir delete",
			batch: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/A", IsDir: true},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/A/B", IsDir: true},
			},
			expected: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/A", IsDir: true},
			},
		},
		{
			name: "Keep unrelated deletes",
			batch: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/A", IsDir: true},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/B", IsDir: true},
			},
			expected: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/A", IsDir: true},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/B", IsDir: true},
			},
		},
		{
			name: "Keep non-delete events",
			batch: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/A", IsDir: true},
				{OSFileEvent: engine.OSFileEvent{Action: "create"}, RemotePath: "/A/new_file.txt", IsDir: false},
			},
			expected: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/A", IsDir: true},
				{OSFileEvent: engine.OSFileEvent{Action: "create"}, RemotePath: "/A/new_file.txt", IsDir: false},
			},
		},
		{
			name: "Avoid prefix-only match (Folder vs FolderSuffix)",
			batch: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/Folder", IsDir: true},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/FolderSuffix/file.txt", IsDir: false},
			},
			expected: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/Folder", IsDir: true},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/FolderSuffix/file.txt", IsDir: false},
			},
		},
		{
			name: "Complex mixed batch",
			batch: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "create"}, RemotePath: "/NewFolder/f1", IsDir: false},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/DelFolder", IsDir: true},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/DelFolder/sub", IsDir: true},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/DelFolder/f2", IsDir: false},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/Other", IsDir: false},
			},
			expected: []engine.UpTaskEvent{
				{OSFileEvent: engine.OSFileEvent{Action: "create"}, RemotePath: "/NewFolder/f1", IsDir: false},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/DelFolder", IsDir: true},
				{OSFileEvent: engine.OSFileEvent{Action: "delete"}, RemotePath: "/Other", IsDir: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := e.deduplicateBatch(tt.batch)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("deduplicateBatch() = %v, want %v", got, tt.expected)
			}
		})
	}
}
