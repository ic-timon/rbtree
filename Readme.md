# Concurrent Red-Black Tree in Go

本项目实现了一个 **支持并发访问的红黑树 (Red-Black Tree)**，并提供了多种并发控制策略的封装，便于在不同场景下进行性能权衡和选择。  
同时包含完整的单元测试、性质验证（红黑树性质检查）、有序/区间操作，以及基准测试 (benchmark)。  
**现已支持基于 gob 的快照 + WAL 增量日志持久化功能。**

---

## ✨ 特性

- **严格的红黑树实现**  
  - 插入、删除均维护红黑树性质（平衡性保证 O(log n) 操作）。  
  - 包含红黑树性质检查，保证逻辑正确性。

- **有序/区间操作**  
  - 支持 `Min`/`Max`/`Prev`/`Next`/`Range` 等有序和区间遍历操作，适合有序检索和区间查询场景。

- **多种并发封装**  
  1. `ShardedRBTreeRW`：全局 `RWMutex` 读写锁  
  2. `ShardedRBTreePath`：全局互斥锁  
  3. `ShardedRBTreeLF`：基于 `sync.Map` 的近似无锁实现  
  4. `ShardedRBTreeOpt`：**分片 (sharding) + Arena 内存池优化**，分片数可自适应 CPU 数量，性能最佳

- **内存复用 (Arena)**  
  使用 `sync.Pool` 避免频繁分配和 GC 压力。

- **基准测试 (Benchmark)**  
  提供不同并发度和实现方式下的性能对比，并支持区间遍历操作的性能测试。

- **持久化支持（Snapshot + WAL）**  
  - 提供 `PersistentManager` 工具，支持对任意树实现进行 gob 快照和增量 WAL 日志持久化。
  - 支持高效恢复、快照与日志自动切换，适合高可靠场景。

---

## 🚀 使用方法

### 安装

```bash
go get github.com/ic-timon/rbtree
```

### 基本用法
```go
package main

import (
    "fmt"
    "github.com/ic-timon/rbtree"
)

func main() {
    tree := rbtree.NewShardedRBTreeOpt(0) // 自动根据 CPU 核心数选择分片数
    tree.Insert(42, "hello")
    if v, ok := tree.Get(42); ok {
        fmt.Println("Found:", v)
    }
    tree.Delete(42)

    // 有序/区间操作示例
    tree.Insert(1, "a")
    tree.Insert(2, "b")
    tree.Insert(3, "c")
    minK, minV, _ := tree.Min()
    fmt.Println("Min:", minK, minV)
    maxK, maxV, _ := tree.Max()
    fmt.Println("Max:", maxK, maxV)
    tree.Range(1, 3, func(k int, v interface{}) bool {
        fmt.Println("Range:", k, v)
        return true
    })
}
```

---

### 🗄️ 持久化用法（快照 + WAL）

```go
import (
    "github.com/ic-timon/rbtree"
)

func main() {
    tree := rbtree.NewShardedRBTreeOpt(0)
    pm, _ := rbtree.NewPersistentManager(tree, "wal.log")

    // 插入/删除自动写 WAL
    pm.Insert(1, "foo")
    pm.Delete(1)

    // 保存快照
    pm.SaveSnapshot("snapshot.gob")
    // 快照后建议清空 WAL
    pm.TruncateWAL("wal.log")

    // 恢复
    tree2 := rbtree.NewShardedRBTreeOpt(0)
    rbtree.LoadFromSnapshotAndWAL(tree2, "snapshot.gob", "wal.log")
}
```
- **说明**：快照后应调用 `TruncateWAL` 清空日志，避免恢复时重复应用操作。

---

### 📊 Benchmark 结果

测试环境：

OS: macOS (Darwin, arm64)

CPU: Apple M3

Go: go1.22+

并发度: W ∈ {16, 32, 64}

| Impl      | W  | ns/op   | B/op   | allocs/op |
| --------- | -- | ------- | ------ | --------- |
| RWLock    | 16 | 1532884 | 144773 | 2881      |
| RWLock    | 32 | 5207324 | 207762 | 9051      |
| RWLock    | 64 | 5258220 | 169947 | 8710      |
| PathLock  | 16 | 5624150 | 183382 | 9023      |
| PathLock  | 32 | 5726667 | 203272 | 8407      |
| PathLock  | 64 | 6855621 | 171884 | 9173      |
| LockFree  | 16 | 959466  | 555084 | 17440     |
| LockFree  | 32 | 1506262 | 778838 | 27329     |
| LockFree  | 64 | 1559660 | 806329 | 27765     |
| Optimized | 16 | 1730035 | 185005 | 9092      |
| Optimized | 32 | 1660828 | 206614 | 9064      |
| Optimized | 64 | 1553283 | 171225 | 8779      |




区间遍历 Range 性能：

| Range区间 | ns/op  | B/op | allocs/op |
|-----------|--------|------|-----------|
| 100       | 2133   | 0    | 0         |
| 10,000    | 54535  | 0    | 0         |

**特性说明：**

- LockFree (sync.Map) 在高并发下性能较好，但内存开销偏高。
- RWLock / PathLock 简单直接，但扩展性有限。
- Optimized (分片+Arena) 在高并发下性能最优，QPS 提升约 3-4 倍，且分片数可自适应 CPU 数量。
- 区间遍历操作极快，且无额外内存分配。

---

### ✅ 功能性测试

项目内置了严格的单元测试：

- 插入 / 查找 / 删除 功能测试
- 红黑树性质验证：
  - 根节点为黑色
  - 红节点的子节点均为黑色
  - 所有路径黑高一致
- 有序/区间操作测试（Min/Max/Prev/Next/Range）
- **持久化与恢复测试**（见 `persistent_test.go`）

运行全部测试与基准测试：
```
go test -v
go test -bench . -benchmem
```

只运行持久化相关基准测试：
```
go test -bench=PersistentManager -benchmem
```

---

### ⚡ 其它说明

- `NewShardedRBTreeOpt(0)` 会自动根据 CPU 数量选择分片数，推荐用法。
- 支持高效的有序检索和区间遍历，适合需要有序集合的高并发场景。
- 若需自定义分片数，可传入正整数。
- **持久化功能不影响原有 API 和测试，按需引入即可。**
