package common

import (
	"container/heap"

	pb "github.com/jdeepd/dron-poc/dron_poc"
)

type PriorityQueue []*pb.Task

func (pq *PriorityQueue) Len() int { return len(*pq) }
func (pq *PriorityQueue) Less(i, j int) bool {
	// TODO
	return true
}
func (pq *PriorityQueue) Pop() interface{} {
	return nil
}

func (pq *PriorityQueue) Push(x interface{}) {
	// TODO
}

func (pq *PriorityQueue) Swap(i, j int) {
	// TODO
}

func createHeap(pq PriorityQueue) {
	heap.Init(&pq)
}
