package rbtree

import (
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"testing"
	"time"
	"unsafe"
)

// 模拟较大 value，能更真实地反映 allocs/B/op
type Value struct {
	Payload [256]byte
}

type Tree interface {
	Insert(int, interface{})
	Get(int) (interface{}, bool)
	Delete(int)
}

// ----------------- 工具：中序遍历 & 排序检查 -----------------
func inorder(n *node, keys *[]int) {
	if n == nil {
		return
	}
	inorder(n.left, keys)
	*keys = append(*keys, n.key)
	inorder(n.right, keys)
}

func isSorted(keys []int) bool {
	return sort.IntsAreSorted(keys)
}

// ----------------- 红黑树性质检查 -----------------
// validateNode 返回 (blackHeight, ok)
func validateNode(n *node) (int, bool) {
	if n == nil {
		// 将 nil 视为黑节点，black-height = 1（或可视为0，和实现一致即可）
		return 1, true
	}

	// 红节点不能有红孩子
	if n.color == red {
		if (n.left != nil && n.left.color == red) || (n.right != nil && n.right.color == red) {
			return 0, false
		}
	}

	lbh, lok := validateNode(n.left)
	rbh, rok := validateNode(n.right)
	if !lok || !rok {
		return 0, false
	}
	if lbh != rbh {
		return 0, false
	}

	if n.color == black {
		return lbh + 1, true
	}
	return lbh, true
}

func checkRBProperties(t *testing.T, root *node) {
	if root == nil {
		return
	}
	if root.color != black {
		t.Fatalf("root must be black")
	}
	if _, ok := validateNode(root); !ok {
		t.Fatalf("red-black properties violated")
	}
}

// ----------------- 功能性测试（严格） -----------------
func TestRBTreeCorrectness(t *testing.T) {
	arena := newArena()
	tree := NewRBTree(arena)

	// 1) 顺序插入
	N := 1000
	for i := 0; i < N; i++ {
		tree.Insert(i, i*10)
	}

	// 全量查验
	for i := 0; i < N; i++ {
		v, ok := tree.Get(i)
		if !ok {
			t.Fatalf("expected key %d present", i)
		}
		if v.(int) != i*10 {
			t.Fatalf("expected value %d, got %v", i*10, v)
		}
	}

	// 中序遍历有序性
	var keys []int
	inorder(tree.root, &keys)
	if len(keys) != N || !isSorted(keys) {
		t.Fatalf("BST property violated after insert (len=%d) sample=%v", len(keys), keys[:min(20, len(keys))])
	}
	// 红黑性质检查
	checkRBProperties(t, tree.root)

	// 2) 删除一半（删除偶数）
	for i := 0; i < N; i += 2 {
		tree.Delete(i)
	}

	// 检查偶数不存在，奇数存在
	for i := 0; i < N; i++ {
		v, ok := tree.Get(i)
		if i%2 == 0 {
			if ok {
				t.Fatalf("expected key %d deleted, but found %v", i, v)
			}
		} else {
			if !ok || v.(int) != i*10 {
				t.Fatalf("expected key %d->%d, got %v (ok=%v)", i, i*10, v, ok)
			}
		}
	}

	// 中序遍历仍有序且长度为 N/2
	keys = keys[:0]
	inorder(tree.root, &keys)
	if len(keys) != N/2 || !isSorted(keys) {
		t.Fatalf("BST property violated after delete (len=%d) sample=%v", len(keys), keys[:min(20, len(keys))])
	}
	checkRBProperties(t, tree.root)

	// 3) 随机插入 + 随机删除半数，验证余下元素正确性
	tree = NewRBTree(arena)
	rand.Seed(time.Now().UnixNano())
	numOps := 5000
	inserted := make(map[int]int)

	for i := 0; i < numOps; i++ {
		k := rand.Intn(2000)
		v := k * 100
		tree.Insert(k, v)
		inserted[k] = v
	}

	// 随机删除一半
	cnt := 0
	for k := range inserted {
		if cnt%2 == 0 {
			tree.Delete(k)
			delete(inserted, k)
		}
		cnt++
	}

	// 验证剩余都可查
	for k, v := range inserted {
		got, ok := tree.Get(k)
		if !ok || got.(int) != v {
			t.Fatalf("after random ops: expected %d->%d, got %v (ok=%v)", k, v, got, ok)
		}
	}
	// 最后一次红黑性质检查
	checkRBProperties(t, tree.root)
}

// ----------------- 辅助 -----------------
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ----------------- 基准测试 -----------------
func BenchmarkTrees(b *testing.B) {
	impls := map[string]func(int) Tree{
		"RWLock": func(_ int) Tree {
			// 为每个 benchmark 构造独立 RBTree（用 arena 管理节点）
			return &ShardedRBTreeRW{tree: NewRBTree(newArena())}
		},
		"PathLock": func(_ int) Tree {
			return &ShardedRBTreePath{tree: NewRBTree(newArena())}
		},
		"LockFree": func(_ int) Tree {
			return &ShardedRBTreeLF{}
		},
		"Optimized": func(shards int) Tree {
			// 将分片数量设置为 shards，这样与并发 worker 数成比例，减少争用
			return NewShardedRBTreeOpt(shards)
		},
	}

	numCPU := runtime.NumCPU()
	Ws := []int{2 * numCPU, 4 * numCPU, 8 * numCPU}
	fmt.Println("=== Benchmark Results ===")
	fmt.Printf("%-10s %-6s %-12s %-12s %-12s\n", "Impl", "W", "ns/op", "B/op", "allocs/op")

	rand.Seed(time.Now().UnixNano())

	for _, W := range Ws {
		N := W * 1000_000_000 // key 空间 10 亿，减少不同 worker 之间的 key 冲突
		for name, ctor := range impls {
			b.Run(fmt.Sprintf("%s-%d", name, W), func(b *testing.B) {
				tree := ctor(W) // 为 Optimized 传 shards=W，其他构造器会忽略参数
				b.ReportAllocs()
				b.ResetTimer()

				b.RunParallel(func(pb *testing.PB) {
					// 每个并发 worker 使用局部 rng，减少全局 rand 锁竞争对 benchmark 的影响
					r := rand.New(rand.NewSource(time.Now().UnixNano() ^ int64(uintptr(unsafePointer()))))
					for pb.Next() {
						key := r.Intn(N)
						// 使用较大 value 模拟真实负载（会造成 alloc）
						tree.Insert(key, &Value{})
						tree.Get(key)
						tree.Delete(key)
					}
				})
			})
		}
	}
}

// unsafePointer 用于生成 per-goroutine 随机种子的一部分（不直接使用 unsafe.Pointer 值，
// 仅返回 uintptr(&seed) 的低位），避免 import unsafe 在代码里显式出现。

func unsafePointer() uintptr {
	var x int
	return uintptr(unsafe.Pointer(&x))
}
