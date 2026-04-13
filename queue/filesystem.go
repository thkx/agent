package queue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// FileSystemPersistence 基于文件系统的持久化实现
type FileSystemPersistence struct {
	directory string
	mu        sync.RWMutex
	items     map[string]any // 内存缓存
}

// NewFileSystemPersistence 创建文件系统持久化
func NewFileSystemPersistence(directory string) (*FileSystemPersistence, error) {
	// 创建目录
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}

	return &FileSystemPersistence{
		directory: directory,
		items:     make(map[string]any),
	}, nil
}

// Push 推送项目
func (fp *FileSystemPersistence) Push(ctx context.Context, item any) error {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	itemID := uuid.New().String()
	fp.items[itemID] = item

	return nil
}

// Pop 弹出项目
func (fp *FileSystemPersistence) Pop(ctx context.Context) (any, error) {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	if len(fp.items) == 0 {
		return nil, ErrQueueClosed
	}

	// 获取第一个项目
	var itemID string
	var item any
	for k, v := range fp.items {
		itemID = k
		item = v
		break
	}

	delete(fp.items, itemID)
	return item, nil
}

// Persist 将项目持久化到文件
func (fp *FileSystemPersistence) Persist() error {
	fp.mu.RLock()
	defer fp.mu.RUnlock()

	for itemID, item := range fp.items {
		filename := filepath.Join(fp.directory, itemID+".json")

		var data []byte
		var err error

		// 根据类型序列化
		switch v := item.(type) {
		case interface{ Marshal() ([]byte, error) }:
			data, err = v.Marshal()
		default:
			// 默认使用 JSON 序列化
			data, err = fp.marshalItem(v)
		}

		if err != nil {
			return fmt.Errorf("failed to marshal item %s: %w", itemID, err)
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filename, err)
		}
	}

	return nil
}

// Recover 从文件恢复项目
func (fp *FileSystemPersistence) Recover() error {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	files, err := os.ReadDir(fp.directory)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", fp.directory, err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".json" {
			continue
		}

		filepath := filepath.Join(fp.directory, file.Name())
		data, err := os.ReadFile(filepath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", filepath, err)
		}

		// 简单地存储为字节数据
		itemID := file.Name()[:len(file.Name())-5] // 移除 .json
		fp.items[itemID] = data

		// 恢复后删除文件
		_ = os.Remove(filepath)
	}

	return nil
}

// marshalItem 序列化项目（简单实现）
func (fp *FileSystemPersistence) marshalItem(item any) ([]byte, error) {
	// 这里可以添加更多类型的支持
	switch v := item.(type) {
	case []byte:
		return v, nil
	default:
		return nil, fmt.Errorf("unsupported type for marshaling: %T", item)
	}
}

// Clear 清空持久化存储
func (fp *FileSystemPersistence) Clear() error {
	fp.mu.Lock()
	defer fp.mu.Unlock()

	fp.items = make(map[string]any)

	// 删除目录中的所有文件
	files, err := os.ReadDir(fp.directory)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() {
			_ = os.Remove(filepath.Join(fp.directory, file.Name()))
		}
	}

	return nil
}
