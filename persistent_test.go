package rbtree

import (
	"encoding/gob"
	"os"
	"testing"
)

func init() {
	gob.Register(&testValue{})
}

// 用于持久化测试的简单 value 类型
type testValue struct {
	V int
}

func TestPersistentManager_SnapshotAndWAL(t *testing.T) {
	// 文件名
	const walFile = "test_wal.log"
	const snapFile = "test_snapshot.gob"
	defer os.Remove(walFile)
	defer os.Remove(snapFile)

	// 1. 构建原始树并持久化操作
	tree := NewShardedRBTreeOpt(0)
	pm, err := NewPersistentManager(tree, walFile)
	if err != nil {
		t.Fatalf("NewPersistentManager failed: %v", err)
	}

	// 插入数据
	N := 100
	for i := 0; i < N; i++ {
		if err := pm.Insert(i, &testValue{V: i * 10}); err != nil {
			t.Fatalf("Insert WAL failed: %v", err)
		}
	}
	// 删除部分
	for i := 0; i < N; i += 3 {
		if err := pm.Delete(i); err != nil {
			t.Fatalf("Delete WAL failed: %v", err)
		}
	}

	// 检查内存树状态
	for i := 0; i < N; i++ {
		v, ok := pm.Get(i)
		if i%3 == 0 {
			if ok {
				t.Fatalf("expected key %d deleted, but found %v", i, v)
			}
		} else {
			if !ok || v.(*testValue).V != i*10 {
				t.Fatalf("expected key %d->%d, got %v (ok=%v)", i, i*10, v, ok)
			}
		}
	}

	// 2. 保存快照
	if err := pm.SaveSnapshot(snapFile); err != nil {
		t.Fatalf("SaveSnapshot failed: %v", err)
	}
	// 2.1 快照后清空 WAL
	if err := pm.TruncateWAL(walFile); err != nil {
		t.Fatalf("TruncateWAL after snapshot failed: %v", err)
	}

	// 3. 新建树，恢复
	tree2 := NewShardedRBTreeOpt(0)
	if err := LoadFromSnapshotAndWAL(tree2, snapFile, walFile); err != nil {
		t.Fatalf("LoadFromSnapshotAndWAL failed: %v", err)
	}

	// 4. 检查恢复后树状态
	for i := 0; i < N; i++ {
		v, ok := tree2.Get(i)
		if i%3 == 0 {
			if ok {
				t.Fatalf("after restore: expected key %d deleted, but found %v", i, v)
			}
		} else {
			if !ok || v.(*testValue).V != i*10 {
				t.Fatalf("after restore: expected key %d->%d, got %v (ok=%v)", i, i*10, v, ok)
			}
		}
	}

	// 5. 清理WAL后再插入并恢复
	if err := pm.TruncateWAL(walFile); err != nil {
		t.Fatalf("TruncateWAL failed: %v", err)
	}
	// 再插入新数据
	for i := N; i < N+10; i++ {
		if err := pm.Insert(i, &testValue{V: i * 10}); err != nil {
			t.Fatalf("Insert after truncate failed: %v", err)
		}
	}
	// 保存快照
	if err := pm.SaveSnapshot(snapFile); err != nil {
		t.Fatalf("SaveSnapshot2 failed: %v", err)
	}
	// 恢复
	tree3 := NewShardedRBTreeOpt(0)
	if err := LoadFromSnapshotAndWAL(tree3, snapFile, walFile); err != nil {
		t.Fatalf("LoadFromSnapshotAndWAL2 failed: %v", err)
	}
	// 检查新数据
	for i := N; i < N+10; i++ {
		v, ok := tree3.Get(i)
		if !ok || v.(*testValue).V != i*10 {
			t.Fatalf("after restore2: expected key %d->%d, got %v (ok=%v)", i, i*10, v, ok)
		}
	}
}

func BenchmarkPersistentManager_InsertAndSnapshot(b *testing.B) {
	const walFile = "bench_wal.log"
	const snapFile = "bench_snapshot.gob"
	defer os.Remove(walFile)
	defer os.Remove(snapFile)

	tree := NewShardedRBTreeOpt(0)
	pm, err := NewPersistentManager(tree, walFile)
	if err != nil {
		b.Fatalf("NewPersistentManager failed: %v", err)
	}

	N := 10000
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 插入 N 条数据并保存快照
		for k := 0; k < N; k++ {
			if err := pm.Insert(k, &testValue{V: k}); err != nil {
				b.Fatalf("Insert WAL failed: %v", err)
			}
		}
		if err := pm.SaveSnapshot(snapFile); err != nil {
			b.Fatalf("SaveSnapshot failed: %v", err)
		}
		// 清空 WAL
		if err := pm.TruncateWAL(walFile); err != nil {
			b.Fatalf("TruncateWAL failed: %v", err)
		}
		// 删除所有数据
		for k := 0; k < N; k++ {
			if err := pm.Delete(k); err != nil {
				b.Fatalf("Delete WAL failed: %v", err)
			}
		}
	}
}

func BenchmarkPersistentManager_Restore(b *testing.B) {
	const walFile = "bench_wal.log"
	const snapFile = "bench_snapshot.gob"
	defer os.Remove(walFile)
	defer os.Remove(snapFile)

	// 先写入快照和WAL
	tree := NewShardedRBTreeOpt(0)
	pm, err := NewPersistentManager(tree, walFile)
	if err != nil {
		b.Fatalf("NewPersistentManager failed: %v", err)
	}
	N := 10000
	for k := 0; k < N; k++ {
		if err := pm.Insert(k, &testValue{V: k}); err != nil {
			b.Fatalf("Insert WAL failed: %v", err)
		}
	}
	if err := pm.SaveSnapshot(snapFile); err != nil {
		b.Fatalf("SaveSnapshot failed: %v", err)
	}
	if err := pm.TruncateWAL(walFile); err != nil {
		b.Fatalf("TruncateWAL failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree2 := NewShardedRBTreeOpt(0)
		if err := LoadFromSnapshotAndWAL(tree2, snapFile, walFile); err != nil {
			b.Fatalf("LoadFromSnapshotAndWAL failed: %v", err)
		}
	}
}
