package webcache

import "container/heap"

type PriorityQueue []*Entry

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i int, j int) bool {
	return pq[i].Less(pq[j])
}

func (pq PriorityQueue) Swap(i int, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(e interface{}) {
	length := pq.Len()
	entry := e.(*Entry)
	entry.index = length
	*pq = append(*pq, entry)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := old.Len()
	entry := old[n-1]
	entry.index = -1
	*pq = old[0:n-1]
	return entry
}

func (pq *PriorityQueue) Update(entry *Entry) {
	heap.Fix(pq, entry.index)
}
