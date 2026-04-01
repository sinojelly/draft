package yike

import (
	"encoding/json"
	"fmt"
)

// extractYikeString 抽取任何类型为 string 的安全助手函数
// 能够处理 string, float64 (不推荐), 以及 json.Number
func extractYikeString(val interface{}) string {
	if val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case json.Number:
		return v.String()
	}
	return fmt.Sprintf("%v", val)
}

// extractValue 用于从叶子节点获取 ID 数字
func extractValue(data interface{}) int64 {
	switch v := data.(type) {
	case json.Number:
		id, _ := v.Int64()
		return id
	case string:
		var id int64
		fmt.Sscanf(v, "%d", &id)
		return id
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}

// extractIDFromMap 递归深度优先搜索提取 fs_id 或 album_id 等主键 ID
func extractIDFromMap(data interface{}, key string) int64 {
	vMap, ok := data.(map[string]interface{})
	if !ok {
		// 如果当前不是 map，检查是否是列表
		if vList, ok := data.([]interface{}); ok {
			for _, item := range vList {
				if id := extractIDFromMap(item, key); id != 0 {
					return id
				}
			}
		}
		return 0
	}

	// 1. 尝试直接从当前层获取
	if val, ok := vMap[key]; ok {
		if id := extractValue(val); id != 0 {
			return id
		}
	}

	// 尝试备选键名 (fs_id <-> fsid)
	altKey := ""
	if key == "fs_id" {
		altKey = "fsid"
	} else if key == "fsid" {
		altKey = "fs_id"
	}
	if altKey != "" {
		if val, ok := vMap[altKey]; ok {
			if id := extractValue(val); id != 0 {
				return id
			}
		}
	}

	// 2. 深度优先递归搜索所有子节点
	// 优先搜索典型业务字段以提升性能
	priorityKeys := []string{"data", "info", "list"}
	for _, pk := range priorityKeys {
		if child, ok := vMap[pk]; ok {
			if id := extractIDFromMap(child, key); id != 0 {
				return id
			}
		}
	}

	// 搜索其他字段
	for k, child := range vMap {
		// 跳过已搜索的
		isPriority := false
		for _, pk := range priorityKeys {
			if k == pk {
				isPriority = true
				break
			}
		}
		if !isPriority {
			if id := extractIDFromMap(child, key); id != 0 {
				return id
			}
		}
	}

	return 0
}
