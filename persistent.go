package rbtree

import (
	"bufio"
	"encoding/gob"
	"os"
	"sync"
)

type Tree interface {
	Insert(int, interface{})
	Get(int) (interface{}, bool)
	Delete(int)
}

// 支持的操作类型
type walOpType byte

const (
	opInsert walOpType = 1
	opDelete walOpType = 2
)

// WAL 操作记录
type walOp struct {
	Op    walOpType
	Key   int
	Value interface{}
}

// 持久化管理器
type PersistentManager struct {
	tree Tree
	mu   sync.Mutex
	wal  *os.File
	w    *bufio.Writer
}

// 创建持久化管理器，tree为目标树，walPath为WAL日志路径
func NewPersistentManager(tree Tree, walPath string) (*PersistentManager, error) {
	wal, err := os.OpenFile(walPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &PersistentManager{
		tree: tree,
		wal:  wal,
		w:    bufio.NewWriter(wal),
	}, nil
}

// 插入并写WAL
func (pm *PersistentManager) Insert(key int, value interface{}) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.tree.Insert(key, value)
	op := walOp{Op: opInsert, Key: key, Value: value}
	enc := gob.NewEncoder(pm.w)
	if err := enc.Encode(&op); err != nil {
		return err
	}
	return pm.w.Flush()
}

// 删除并写WAL
func (pm *PersistentManager) Delete(key int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.tree.Delete(key)
	op := walOp{Op: opDelete, Key: key}
	enc := gob.NewEncoder(pm.w)
	if err := enc.Encode(&op); err != nil {
		return err
	}
	return pm.w.Flush()
}

// 查询直接透传
func (pm *PersistentManager) Get(key int) (interface{}, bool) {
	return pm.tree.Get(key)
}

// 保存快照
func (pm *PersistentManager) SaveSnapshot(snapshotPath string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	f, err := os.Create(snapshotPath)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := gob.NewEncoder(f)
	data := ExportAll(pm.tree)
	return enc.Encode(data)
}

// 从快照和WAL恢复
func LoadFromSnapshotAndWAL(tree Tree, snapshotPath, walPath string) error {
	// 1. 加载快照
	if _, err := os.Stat(snapshotPath); err == nil {
		f, err := os.Open(snapshotPath)
		if err != nil {
			return err
		}
		defer f.Close()
		dec := gob.NewDecoder(f)
		var data map[int]interface{}
		if err := dec.Decode(&data); err != nil {
			return err
		}
		ImportAll(tree, data)
	}
	// 2. 重放WAL（同原实现）
	if _, err := os.Stat(walPath); err == nil {
		wal, err := os.Open(walPath)
		if err != nil {
			return err
		}
		defer wal.Close()
		dec := gob.NewDecoder(wal)
		for {
			var op walOp
			if err := dec.Decode(&op); err != nil {
				break
			}
			switch op.Op {
			case opInsert:
				tree.Insert(op.Key, op.Value)
			case opDelete:
				tree.Delete(op.Key)
			}
		}
	}
	return nil
}

// 清理WAL（快照后可调用）
func (pm *PersistentManager) TruncateWAL(walPath string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.wal.Close()
	if err := os.Truncate(walPath, 0); err != nil {
		return err
	}
	// 重新打开 WAL 文件和 bufio.Writer
	wal, err := os.OpenFile(walPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	pm.wal = wal
	pm.w = bufio.NewWriter(wal)
	return nil
}

// 导出所有 key-value（快照用）
func ExportAll(tree Tree) map[int]interface{} {
	result := make(map[int]interface{})
	// 适配不同实现
	switch t := tree.(type) {
	case *ShardedRBTreeOpt:
		for _, sh := range t.shards {
			sh.mu.RLock()
			sh.tree.Range(-1<<31, 1<<31-1, func(k int, v interface{}) bool {
				result[k] = v
				return true
			})
			sh.mu.RUnlock()
		}
	case *ShardedRBTreeRW:
		t.mu.RLock()
		t.tree.Range(-1<<31, 1<<31-1, func(k int, v interface{}) bool {
			result[k] = v
			return true
		})
		t.mu.RUnlock()
	case *ShardedRBTreePath:
		t.mu.Lock()
		t.tree.Range(-1<<31, 1<<31-1, func(k int, v interface{}) bool {
			result[k] = v
			return true
		})
		t.mu.Unlock()
	case *ShardedRBTreeLF:
		t.data.Range(func(key, value interface{}) bool {
			result[key.(int)] = value
			return true
		})
	}
	return result
}

// 从快照数据恢复
func ImportAll(tree Tree, data map[int]interface{}) {
	for k, v := range data {
		tree.Insert(k, v)
	}
}
