package microblog

import (
	"container/heap"
	"container/list"

	"golang.org/x/exp/constraints"
)

type MinHeap[K constraints.Ordered] []K

func (h MinHeap[K]) Len() int {
	return len(h)
}

func (h MinHeap[K]) Less(i, j int) bool {
	return h[i] < h[j]
}

func (h MinHeap[K]) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *MinHeap[K]) Push(x any) {
	*h = append(*h, x.(K))
}

func (h *MinHeap[K]) Pop() any {

	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type Pair[K, V any] struct {
	Key K
	Val V
}

type LRU[K constraints.Ordered, V any] struct {
	Dict map[K]*list.Element
	List *list.List
	Heap *MinHeap[K]
}

type ListIterator[K comparable, V any] struct {
	List  *list.List
	Order IterationOrder
	Head  *list.Element
}

func (lst *ListIterator[K, V]) Next() (K, V, error) {
	var key K
	var val V
	if lst.Head == nil {
		return key, val, ErrEmptyIterator
	} else {
		pair := lst.Head.Value.(Pair[K, V])
		key = pair.Key
		val = pair.Val
		if lst.Order == ByInsertion {
			lst.Head = lst.Head.Next()
		} else if lst.Order == ByInsertionRev {
			lst.Head = lst.Head.Prev()
		}
		return key, val, nil
	}
}

func (lst *ListIterator[K, V]) HasNext() bool {
	if lst.Head != nil {
		return true
	}
	return false
}

func NewCollection[K constraints.Ordered, V any]() *LRU[K, V] {
	col := LRU[K, V]{}
	col.Dict = make(map[K]*list.Element)
	col.List = list.New()
	h := &MinHeap[K]{}
	heap.Init(h)
	col.Heap = h
	return &col
}

func (col *LRU[K, V]) Len() int {
	return col.List.Len()
}

func (col *LRU[K, V]) Add(key K, value V) error {
	if _, ok := col.Dict[key]; ok {
		return ErrDuplicateKey
	} else {
		el := col.List.PushBack(Pair[K, V]{key, value})
		col.Dict[key] = el
		heap.Push(col.Heap, key)
	}
	return nil
}

func (col *LRU[K, V]) DelMin() (K, V, error) {
	var v V
	var key K
	if col.Len() == 0 {
		return key, v, ErrEmptyCollection
	}
	key = heap.Pop(col.Heap).(K)
	v = col.Dict[key].Value.(Pair[K, V]).Val
	col.List.Remove(col.Dict[key])
	delete(col.Dict, key)
	return key, v, nil
}

func (col *LRU[K, V]) At(key K) (V, bool) {
	var v V
	el, ok := col.Dict[key]
	if ok {
		pair := el.Value.(Pair[K, V])
		return pair.Val, ok
	}
	return v, ok
}

func (col *LRU[K, V]) IterateBy(order IterationOrder) Iterator[K, V] {
	var it ListIterator[K, V]
	if order == ByInsertion {
		it = ListIterator[K, V]{col.List, order, col.List.Front()}
	} else if order == ByInsertionRev {
		it = ListIterator[K, V]{col.List, order, col.List.Back()}
	} else {
		panic(ErrUnknownOrder)
	}
	return &it
}
