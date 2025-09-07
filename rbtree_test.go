package rbtree

import (
	"fmt"
	"math/rand"
	"runtime"
	"sort"
	"testing"
	"time"
)

// 模拟较大 value，能更真实地反映 allocs/B/op
type Value struct {
	Payload [1]byte
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

// ----------------- 有序/区间操作功能测试 -----------------
func TestRBTreeOrderOps(t *testing.T) {
	arena := newArena()
	tree := NewRBTree(arena)
	N := 1000
	for i := 0; i < N; i++ {
		tree.Insert(i, i*10)
	}

	// Min/Max
	minK, minV, ok := tree.Min()
	if !ok || minK != 0 || minV.(int) != 0 {
		t.Fatalf("Min failed: got %v %v", minK, minV)
	}
	maxK, maxV, ok := tree.Max()
	if !ok || maxK != N-1 || maxV.(int) != (N-1)*10 {
		t.Fatalf("Max failed: got %v %v", maxK, maxV)
	}

	// Prev/Next
	for i := 1; i < N-1; i++ {
		pk, pv, ok := tree.Prev(i)
		if !ok || pk != i-1 || pv.(int) != (i-1)*10 {
			t.Fatalf("Prev(%d) failed: got %v %v", i, pk, pv)
		}
		nk, nv, ok := tree.Next(i)
		if !ok || nk != i+1 || nv.(int) != (i+1)*10 {
			t.Fatalf("Next(%d) failed: got %v %v", i, nk, nv)
		}
	}
	// Prev of min
	_, _, ok = tree.Prev(0)
	if ok {
		t.Fatalf("Prev(0) should not exist")
	}
	// Next of max
	_, _, ok = tree.Next(N - 1)
	if ok {
		t.Fatalf("Next(N-1) should not exist")
	}

	// 区间遍历
	sum := 0
	tree.Range(100, 199, func(k int, v interface{}) bool {
		sum += k
		return true
	})
	expect := 0
	for i := 100; i <= 199; i++ {
		expect += i
	}
	if sum != expect {
		t.Fatalf("Range sum failed: got %d, want %d", sum, expect)
	}
}

// ----------------- 并发封装有序/区间操作功能测试 -----------------
func TestShardedRBTreeOptOrderOps(t *testing.T) {
	tree := NewShardedRBTreeOpt(0)
	N := 1000
	for i := 0; i < N; i++ {
		tree.Insert(i, i*10)
	}
	minK, minV, ok := tree.Min()
	if !ok || minK != 0 || minV.(int) != 0 {
		t.Fatalf("Min failed: got %v %v", minK, minV)
	}
	maxK, maxV, ok := tree.Max()
	if !ok || maxK != N-1 || maxV.(int) != (N-1)*10 {
		t.Fatalf("Max failed: got %v %v", maxK, maxV)
	}
	sum := 0
	tree.Range(100, 199, func(k int, v interface{}) bool {
		sum += k
		return true
	})
	expect := 0
	for i := 100; i <= 199; i++ {
		expect += i
	}
	if sum != expect {
		t.Fatalf("Range sum failed: got %d, want %d", sum, expect)
	}
}

func TestShardedRBTreeRWOrderOps(t *testing.T) {
	tree := &ShardedRBTreeRW{tree: NewRBTree(newArena())}
	N := 1000
	for i := 0; i < N; i++ {
		tree.Insert(i, i*10)
	}
	minK, minV, ok := tree.Min()
	if !ok || minK != 0 || minV.(int) != 0 {
		t.Fatalf("Min failed: got %v %v", minK, minV)
	}
	maxK, maxV, ok := tree.Max()
	if !ok || maxK != N-1 || maxV.(int) != (N-1)*10 {
		t.Fatalf("Max failed: got %v %v", maxK, maxV)
	}
	sum := 0
	tree.Range(100, 199, func(k int, v interface{}) bool {
		sum += k
		return true
	})
	expect := 0
	for i := 100; i <= 199; i++ {
		expect += i
	}
	if sum != expect {
		t.Fatalf("Range sum failed: got %d, want %d", sum, expect)
	}
}

func TestShardedRBTreePathOrderOps(t *testing.T) {
	tree := &ShardedRBTreePath{tree: NewRBTree(newArena())}
	N := 1000
	for i := 0; i < N; i++ {
		tree.Insert(i, i*10)
	}
	minK, minV, ok := tree.Min()
	if !ok || minK != 0 || minV.(int) != 0 {
		t.Fatalf("Min failed: got %v %v", minK, minV)
	}
	maxK, maxV, ok := tree.Max()
	if !ok || maxK != N-1 || maxV.(int) != (N-1)*10 {
		t.Fatalf("Max failed: got %v %v", maxK, maxV)
	}
	sum := 0
	tree.Range(100, 199, func(k int, v interface{}) bool {
		sum += k
		return true
	})
	expect := 0
	for i := 100; i <= 199; i++ {
		expect += i
	}
	if sum != expect {
		t.Fatalf("Range sum failed: got %d, want %d", sum, expect)
	}
}

// ----------------- 辅助 -----------------
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ----------------- 并发基准测试（阶段性：插入 -> 查询 -> 删除） -----------------
func BenchmarkTrees(b *testing.B) {
	impls := map[string]func(int) Tree{
		"RWLock": func(_ int) Tree {
			return &ShardedRBTreeRW{tree: NewRBTree(newArena())}
		},
		"PathLock": func(_ int) Tree {
			return &ShardedRBTreePath{tree: NewRBTree(newArena())}
		},
		"LockFree": func(_ int) Tree {
			return &ShardedRBTreeLF{}
		},
		"Optimized": func(shards int) Tree {
			return NewShardedRBTreeOpt(shards)
		},
	}

	numCPU := runtime.NumCPU()
	Ws := []int{2 * numCPU, 4 * numCPU, 8 * numCPU}

	fmt.Println("=== Benchmark Results (Concurrent Insert -> Get -> Delete) ===")
	fmt.Printf("%-10s %-6s %-12s %-12s %-12s\n", "Impl", "W", "ns/op", "B/op", "allocs/op")

	rand.Seed(time.Now().UnixNano())

	for _, W := range Ws {
		N := W * 1_000 // 每阶段操作总数
		keys := make([]int, N)
		for i := 0; i < N; i++ {
			keys[i] = rand.Intn(N * 10)
		}

		for name, ctor := range impls {
			b.Run(fmt.Sprintf("%s-%d", name, W), func(b *testing.B) {
				tree := ctor(W)
				b.ReportAllocs()
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					// -------- 并发插入 --------
					b.RunParallel(func(pb *testing.PB) {
						r := rand.New(rand.NewSource(time.Now().UnixNano()))
						for pb.Next() {
							k := keys[r.Intn(len(keys))]
							tree.Insert(k, &Value{})
						}
					})

					// -------- 并发查询 --------
					b.RunParallel(func(pb *testing.PB) {
						r := rand.New(rand.NewSource(time.Now().UnixNano()))
						for pb.Next() {
							k := keys[r.Intn(len(keys))]
							tree.Get(k)
						}
					})

					// -------- 并发删除 --------
					b.RunParallel(func(pb *testing.PB) {
						r := rand.New(rand.NewSource(time.Now().UnixNano()))
						for pb.Next() {
							k := keys[r.Intn(len(keys))]
							tree.Delete(k)
						}
					})
				}
			})
		}
	}
}

// ----------------- 区间遍历基准测试 -----------------
func BenchmarkRangeOps(b *testing.B) {
	tree := NewShardedRBTreeOpt(0)
	N := 1_000_000
	for i := 0; i < N; i++ {
		tree.Insert(i, i)
	}
	b.ResetTimer()
	b.Run("Range-100", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sum := 0
			tree.Range(100, 199, func(k int, v interface{}) bool {
				sum += k
				return true
			})
		}
	})
	b.Run("Range-10k", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			sum := 0
			tree.Range(100_000, 109_999, func(k int, v interface{}) bool {
				sum += k
				return true
			})
		}
	})
}
