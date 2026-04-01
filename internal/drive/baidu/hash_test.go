package baidu

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateHashes(t *testing.T) {
	tempDir := t.TempDir()

	tinyFile := filepath.Join(tempDir, "tiny.txt")
	err := os.WriteFile(tinyFile, []byte("hello world syncghost v1"), 0644)
	if err != nil {
		t.Fatalf("Failed to write tiny file: %v", err)
	}

	hashes, err := CalculateHashes(tinyFile)
	if err != nil {
		t.Fatalf("CalculateHashes failed on tiny file: %v", err)
	}

	if hashes.Size != 24 {
		t.Errorf("Expected Size 24, got %d", hashes.Size)
	}
	if hashes.MD5 == "" || hashes.SliceMD5 == "" || hashes.CRC32 == "" {
		t.Errorf("Hashes are dangerously empty for tiny file")
	}
	if len(hashes.BlockList) != 1 {
		t.Errorf("Tiny file should have exactly 1 block hash, got %d", len(hashes.BlockList))
	}

	heavyFile := filepath.Join(tempDir, "heavy.bin")
	f, err := os.Create(heavyFile)
	if err != nil {
		t.Fatalf("Failed to create heavy file: %v", err)
	}

	// Create ~5MB file to test slicing
	chunkData := make([]byte, 5*1024*1024)
	rand.Read(chunkData)
	f.Write(chunkData)
	f.Close()

	hashesL, err := CalculateHashes(heavyFile)
	if err != nil {
		t.Fatalf("CalculateHashes failed on heavy file: %v", err)
	}

	if hashesL.Size != 5*1024*1024 {
		t.Errorf("Expected 5MB Size, got %d", hashesL.Size)
	}
	if hashesL.SliceMD5 == hashesL.MD5 {
		t.Errorf("SliceMD5 should not equal Full MD5 on files >256KB")
	}
	if len(hashesL.BlockList) != 1 {
		t.Errorf("5MB file should have exactly 1 block list mapped to 32MB constraint, got %d", len(hashesL.BlockList))
	}
}
