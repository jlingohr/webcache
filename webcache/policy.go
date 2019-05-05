package webcache

import (
	"container/heap"
	"container/list"
	"log"
)

type Policy interface {
	Promote(entry *Entry)
	Evict() *Entry //Clear size bytes from cache
}

type LRUPolicy struct {
	entries *list.List
}

func NewLRUPolicy() *LRUPolicy {
	return &LRUPolicy{list.New()}
}


func (l *LRUPolicy) Promote(entry *Entry)  {
	if entry.element != nil {
		l.entries.MoveToFront(entry.element)
	} else {
		entry.element = l.entries.PushFront(entry)
	}
}

func (l *LRUPolicy) Evict() *Entry {
	item := l.entries.Back()
	if item == nil { return nil }
	entry := item.Value.(*Entry)
	l.entries.Remove(item)
	log.Printf("LRU - evict %s", entry.Key)
	return entry
}

// *** Doesn't seem like this is being used?
//func (l *LRUPolicy) Delete(entry *Entry) {
//	l.entries.Remove(entry.element)
//}

/////////////
// LFU

type LFUPolicy struct {
	entries *PriorityQueue
	tick uint64
}

func NewLFUPolicy() *LFUPolicy {
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)
	return &LFUPolicy{&pq, 0}
}


func (l *LFUPolicy) Promote(entry *Entry)  {
	l.tick += 1
	if entry.index < 0 {
		entry.hits += 1
		entry.tick = l.tick
		l.entries.Push(entry)
	} else {
		entry.hits += 1
		entry.tick += 1
		l.entries.Update(entry)
	}
}

func (l *LFUPolicy) Evict() *Entry {
	//TODO
	entry := heap.Pop(l.entries).(*Entry)
	log.Printf("LFU - evict %s. Frequency is %d", entry.Key, entry.hits)
	return entry
}
