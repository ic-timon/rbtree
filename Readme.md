# Concurrent Red-Black Tree in Go

本项目实现了一个 **支持并发访问的红黑树 (Red-Black Tree)**，并提供了多种并发控制策略的封装，便于在不同场景下进行性能权衡和选择。  
同时包含完整的单元测试、性质验证（红黑树性质检查）、以及基准测试 (benchmark)。

---

## ✨ 特性

- **严格的红黑树实现**  
  - 插入、删除均维护红黑树性质（平衡性保证 O(log n) 操作）。  
  - 包含红黑树性质检查，保证逻辑正确性。

- **多种并发封装**  
  1. `ShardedRBTreeRW`：全局 `RWMutex` 读写锁  
  2. `ShardedRBTreePath`：全局互斥锁  
  3. `ShardedRBTreeLF`：基于 `sync.Map` 的近似无锁实现  
  4. `ShardedRBTreeOpt`：**分片 (sharding) + Arena 内存池优化**，性能最佳

- **内存复用 (Arena)**  
  使用 `sync.Pool` 避免频繁分配和 GC 压力。

- **基准测试 (Benchmark)**  
  提供不同并发度和实现方式下的性能对比。

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
}

```

---
### 📊 Benchmark 结果

测试环境：

OS: macOS (Darwin, arm64)

CPU: Apple M3

Go: go1.22+

并发度: W ∈ {16, 32, 64}

| Impl      | W  | ns/op | B/op | allocs/op |
| --------- | -- | ----- | ---- | --------- |
| RWLock    | 16 | 275.0 | 256  | 1         |
| PathLock  | 16 | 299.7 | 256  | 1         |
| LockFree  | 16 | 205.4 | 321  | 3         |
| Optimized | 16 | 84.41 | 256  | 1         |
| RWLock    | 32 | 267.9 | 256  | 1         |
| PathLock  | 32 | 302.9 | 256  | 1         |
| LockFree  | 32 | 204.6 | 321  | 3         |
| Optimized | 32 | 69.84 | 256  | 1         |
| RWLock    | 64 | 267.4 | 256  | 1         |
| PathLock  | 64 | 298.8 | 256  | 1         |
| LockFree  | 64 | 204.9 | 321  | 3         |
| Optimized | 64 | 64.89 | 256  | 1         |

特性：

LockFree (sync.Map) 在中等负载下性能较好，但内存开销偏高。

RWLock / PathLock 简单直接，但扩展性有限。

Optimized (分片+Arena) 在高并发下性能最优，QPS 提升约 3-4 倍。


---
### ✅ 功能性测试

项目内置了严格的单元测试：

插入 / 查找 / 删除 功能测试

红黑树性质验证：

根节点为黑色

红节点的子节点均为黑色

所有路径黑高一致

运行基准测试：
```
go test -bench . -benchmem
```
