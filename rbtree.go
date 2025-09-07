package rbtree

import (
	"runtime"
	"sync"
)

type color bool

const (
	red   color = true
	black color = false
)

// ================= 节点定义 =================
type node struct {
	key    int
	value  interface{}
	color  color
	left   *node
	right  *node
	parent *node
}

// ================= Arena 分配器 =================
type arena struct {
	pool sync.Pool
}

func newArena() *arena {
	return &arena{
		pool: sync.Pool{
			New: func() interface{} { return new(node) },
		},
	}
}

func (a *arena) newNode(key int, value interface{}) *node {
	n := a.pool.Get().(*node)
	n.key = key
	n.value = value
	n.left, n.right, n.parent = nil, nil, nil
	n.color = red
	return n
}

func (a *arena) freeNode(n *node) {
	if n == nil {
		return
	}
	// 避免内存泄露
	n.left, n.right, n.parent, n.value = nil, nil, nil, nil
	a.pool.Put(n)
}

// ================= 红黑树 =================
type RBTree struct {
	root  *node
	arena *arena
}

func NewRBTree(a *arena) *RBTree {
	return &RBTree{arena: a}
}

func getColor(n *node) color {
	if n == nil {
		return black
	}
	return n.color
}

func (t *RBTree) minimum(x *node) *node {
	for x.left != nil {
		x = x.left
	}
	return x
}

func (t *RBTree) transplant(u, v *node) {
	if u.parent == nil {
		t.root = v
	} else if u == u.parent.left {
		u.parent.left = v
	} else {
		u.parent.right = v
	}
	if v != nil {
		v.parent = u.parent
	}
}

func (t *RBTree) rotateLeft(x *node) {
	y := x.right
	x.right = y.left
	if y.left != nil {
		y.left.parent = x
	}
	y.parent = x.parent
	if x.parent == nil {
		t.root = y
	} else if x == x.parent.left {
		x.parent.left = y
	} else {
		x.parent.right = y
	}
	y.left = x
	x.parent = y
}

func (t *RBTree) rotateRight(x *node) {
	y := x.left
	x.left = y.right
	if y.right != nil {
		y.right.parent = x
	}
	y.parent = x.parent
	if x.parent == nil {
		t.root = y
	} else if x == x.parent.right {
		x.parent.right = y
	} else {
		x.parent.left = y
	}
	y.right = x
	x.parent = y
}

func (t *RBTree) Insert(key int, value interface{}) {
	var y *node
	x := t.root
	for x != nil {
		y = x
		if key < x.key {
			x = x.left
		} else if key > x.key {
			x = x.right
		} else {
			x.value = value
			return
		}
	}
	z := t.arena.newNode(key, value)
	z.parent = y
	if y == nil {
		t.root = z
	} else if z.key < y.key {
		y.left = z
	} else {
		y.right = z
	}
	t.insertFixup(z)
}

func (t *RBTree) insertFixup(z *node) {
	for z.parent != nil && z.parent.color == red {
		if z.parent == z.parent.parent.left {
			y := z.parent.parent.right
			if getColor(y) == red {
				z.parent.color = black
				y.color = black
				z.parent.parent.color = red
				z = z.parent.parent
			} else {
				if z == z.parent.right {
					z = z.parent
					t.rotateLeft(z)
				}
				z.parent.color = black
				z.parent.parent.color = red
				t.rotateRight(z.parent.parent)
			}
		} else {
			y := z.parent.parent.left
			if getColor(y) == red {
				z.parent.color = black
				y.color = black
				z.parent.parent.color = red
				z = z.parent.parent
			} else {
				if z == z.parent.left {
					z = z.parent
					t.rotateRight(z)
				}
				z.parent.color = black
				z.parent.parent.color = red
				t.rotateLeft(z.parent.parent)
			}
		}
	}
	t.root.color = black
}

func (t *RBTree) Get(key int) (interface{}, bool) {
	x := t.root
	for x != nil {
		if key < x.key {
			x = x.left
		} else if key > x.key {
			x = x.right
		} else {
			return x.value, true
		}
	}
	return nil, false
}

func (t *RBTree) Delete(key int) {
	z := t.root
	for z != nil {
		if key < z.key {
			z = z.left
		} else if key > z.key {
			z = z.right
		} else {
			break
		}
	}
	if z == nil {
		return
	}

	y := z
	yOrigColor := y.color
	var x *node
	var xParent *node

	if z.left == nil {
		x = z.right
		xParent = z.parent
		t.transplant(z, z.right)
	} else if z.right == nil {
		x = z.left
		xParent = z.parent
		t.transplant(z, z.left)
	} else {
		y = t.minimum(z.right)
		yOrigColor = y.color
		x = y.right
		if y.parent == z {
			xParent = y
		} else {
			t.transplant(y, y.right)
			y.right = z.right
			y.right.parent = y
			xParent = y.parent
		}
		t.transplant(z, y)
		y.left = z.left
		y.left.parent = y
		y.color = z.color
	}
	if yOrigColor == black {
		t.deleteFixup(x, xParent)
	}
	t.arena.freeNode(z)
}

func (t *RBTree) deleteFixup(x *node, parent *node) {
	for (x != t.root) && getColor(x) == black {
		if parent == nil {
			break
		}
		if x == parent.left {
			w := parent.right
			if getColor(w) == red {
				w.color = black
				parent.color = red
				t.rotateLeft(parent)
				w = parent.right
			}
			if getColor(w.left) == black && getColor(w.right) == black {
				w.color = red
				x = parent
				parent = x.parent
			} else {
				if getColor(w.right) == black {
					if w.left != nil {
						w.left.color = black
					}
					w.color = red
					t.rotateRight(w)
					w = parent.right
				}
				w.color = parent.color
				parent.color = black
				if w.right != nil {
					w.right.color = black
				}
				t.rotateLeft(parent)
				x = t.root
				break
			}
		} else {
			w := parent.left
			if getColor(w) == red {
				w.color = black
				parent.color = red
				t.rotateRight(parent)
				w = parent.left
			}
			if getColor(w.right) == black && getColor(w.left) == black {
				w.color = red
				x = parent
				parent = x.parent
			} else {
				if getColor(w.left) == black {
					if w.right != nil {
						w.right.color = black
					}
					w.color = red
					t.rotateLeft(w)
					w = parent.left
				}
				w.color = parent.color
				parent.color = black
				if w.left != nil {
					w.left.color = black
				}
				t.rotateRight(parent)
				x = t.root
				break
			}
		}
	}
	if x != nil {
		x.color = black
	}
}

// ================= 并发封装 =================

// 1. 全局 RWLock
type ShardedRBTreeRW struct {
	tree *RBTree
	mu   sync.RWMutex
}

func (s *ShardedRBTreeRW) Insert(key int, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tree.Insert(key, value)
}
func (s *ShardedRBTreeRW) Get(key int) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tree.Get(key)
}
func (s *ShardedRBTreeRW) Delete(key int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tree.Delete(key)
}

// 2. 全局 PathLock
type ShardedRBTreePath struct {
	tree *RBTree
	mu   sync.Mutex
}

func (s *ShardedRBTreePath) Insert(key int, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tree.Insert(key, value)
}
func (s *ShardedRBTreePath) Get(key int) (interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tree.Get(key)
}
func (s *ShardedRBTreePath) Delete(key int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tree.Delete(key)
}

// 3. LockFree sync.Map
type ShardedRBTreeLF struct {
	data sync.Map
}

func (s *ShardedRBTreeLF) Insert(key int, value interface{}) {
	s.data.Store(key, value)
}
func (s *ShardedRBTreeLF) Get(key int) (interface{}, bool) {
	return s.data.Load(key)
}
func (s *ShardedRBTreeLF) Delete(key int) {
	s.data.Delete(key)
}

// 4. Optimized 分片
type shard struct {
	tree *RBTree
	mu   sync.RWMutex
}

type ShardedRBTreeOpt struct {
	shards []*shard
	arena  *arena
}

func NewShardedRBTreeOpt(shardsNum int) *ShardedRBTreeOpt {
	if shardsNum <= 0 {
		shardsNum = runtime.NumCPU() * 8
	}
	a := newArena()
	shards := make([]*shard, shardsNum)
	for i := range shards {
		shards[i] = &shard{tree: NewRBTree(a)}
	}
	return &ShardedRBTreeOpt{shards: shards, arena: a}
}

func (s *ShardedRBTreeOpt) getShard(key int) *shard {
	idx := key % len(s.shards)
	if idx < 0 {
		idx += len(s.shards)
	}
	return s.shards[idx]
}

func (s *ShardedRBTreeOpt) Insert(key int, value interface{}) {
	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.tree.Insert(key, value)
}
func (s *ShardedRBTreeOpt) Get(key int) (interface{}, bool) {
	sh := s.getShard(key)
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return sh.tree.Get(key)
}
func (s *ShardedRBTreeOpt) Delete(key int) {
	sh := s.getShard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.tree.Delete(key)
}
