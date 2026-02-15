package common

import (
	"container/heap"

	pb "github.com/jdeepd/dron-poc/dron_poc"
	"github.com/rs/zerolog/log"
)

type PriorityQueue []*pb.Task

func (pq *PriorityQueue) Len() int { return len(*pq) }

func (pq *PriorityQueue) Less(i, j int) bool {
	return (*pq)[i].Priority > (*pq)[j].Priority
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

func (pq *PriorityQueue) Push(x interface{}) {
	task, ok := x.(*pb.Task)
	if !ok {
		log.Fatal().Msg("Invalid type for PriorityQueue")
		return
	}
	*pq = append(*pq, task)
}

func (pq *PriorityQueue) Swap(i, j int) {
	(*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i]
}

func createHeap(pq PriorityQueue) {
	heap.Init(&pq)
}
