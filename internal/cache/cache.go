// Package cache 提供带 TTL 的 JSON 文件缓存（PRD 数据刷新架构：无常驻进程，
// 每次调用先读缓存，过期才重算并回写）。
package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"ccpill/internal/config"
)

type envelope struct {
	SavedAt int64           `json:"saved_at"` // Unix 秒
	Data    json.RawMessage `json:"data"`
}

func pathFor(key string) string {
	return filepath.Join(config.Dir(), "cache", key+".json")
}

// Get 读取缓存；不存在/过期/损坏均返回 false。
func Get(key string, ttl time.Duration, v any) bool {
	b, err := os.ReadFile(pathFor(key))
	if err != nil {
		return false
	}
	var env envelope
	if json.Unmarshal(b, &env) != nil {
		return false
	}
	if time.Since(time.Unix(env.SavedAt, 0)) > ttl {
		return false
	}
	return json.Unmarshal(env.Data, v) == nil
}

// Put 写入缓存（尽力而为，失败静默——缓存缺失只是慢，不能影响渲染）。
func Put(key string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	b, err := json.Marshal(envelope{SavedAt: time.Now().Unix(), Data: data})
	if err != nil {
		return
	}
	dir := filepath.Dir(pathFor(key))
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, key+"-*")
	if err != nil {
		return
	}
	if _, err := tmp.Write(b); err == nil && tmp.Close() == nil {
		_ = os.Rename(tmp.Name(), pathFor(key))
		return
	}
	tmp.Close()
	_ = os.Remove(tmp.Name())
}
