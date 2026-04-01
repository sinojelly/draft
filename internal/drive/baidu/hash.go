package baidu

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

// The slice length required by Baidu for the precreate endpoint
const SliceLength = 256 * 1024     // 256KB
const BlockSize = 32 * 1024 * 1024 // 32MB max per chunk

// FileHashes stores the three hashes required by Baidu PCS
type FileHashes struct {
	MD5       string
	SliceMD5  string
	CRC32     string
	Size      int64
	BlockList []string
}

// CalculateHashes computes full MD5, first 256KB MD5, CRC32, and block level MD5s in a single pass.
func CalculateHashes(localPath string) (*FileHashes, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()

	md5Hash := md5.New()
	crc32Hash := crc32.NewIEEE()

	var blockList []string
	var sliceMd5Str string

	buf := make([]byte, BlockSize) // 复用 4MB 的内存块
	isFirstBlock := true

	for {
		n, err := file.Read(buf)
		if n > 0 {
			// 1. 累加计算 Full-MD5 & CRC32
			writeBuf := buf[:n]
			md5Hash.Write(writeBuf)
			crc32Hash.Write(writeBuf)

			// 2. 计算当前 4MB 分片的 Block-MD5 (API limits Block to 4MB locally mapped to 32MB max via our constant)
			bHash := md5.Sum(writeBuf)
			blockList = append(blockList, hex.EncodeToString(bHash[:]))

			// 3. 计算首个 256KB 的 Slice-MD5
			if isFirstBlock {
				sliceLen := SliceLength
				if n < sliceLen {
					sliceLen = n // 兼容极小文件
				}
				sliceHash := md5.Sum(buf[:sliceLen])
				sliceMd5Str = hex.EncodeToString(sliceHash[:])
				isFirstBlock = false
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	// Baidu PCS specific 0-byte/small file handling:
	// If the file is 0 bytes, we must still provide one block (the MD5 of empty)
	if size == 0 {
		emptyMd5 := "d41d8cd98f00b204e9800998ecf8427e"
		return &FileHashes{
			MD5:       emptyMd5,
			SliceMD5:  emptyMd5,
			CRC32:     "0",
			Size:      0,
			BlockList: []string{emptyMd5},
		}, nil
	}

	// Normal case result construction
	resMD5 := hex.EncodeToString(md5Hash.Sum(nil))
	if sliceMd5Str == "" {
		sliceMd5Str = resMD5
	}

	return &FileHashes{
		MD5:       resMD5,
		SliceMD5:  sliceMd5Str,
		CRC32:     fmt.Sprintf("%v", crc32Hash.Sum32()),
		Size:      size,
		BlockList: blockList,
	}, nil
}

// CalculateMD5 computes a simple hex MD5 of a file for download verification
func CalculateMD5(localPath string) (string, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := md5.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
