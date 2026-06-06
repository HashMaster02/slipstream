package datastructures

import (
	"container/list"

	"github.com/HashMaster02/slipstream/internal/events"
)

type Queue struct {
	items *list.List
}

func NewQueue() Queue {
	return Queue{list.New()}
}

func (q *Queue) Push(e events.Event) {
	q.items.PushBack(e)
}

func (q *Queue) Pop() (events.Event, bool) {

	item := q.items.Front()

	if item != nil {
		q.items.Remove(item)

		if e, ok := item.Value.(events.Event); ok {
			return e, true
		}
	}

	return nil, false
}

func (q *Queue) Len() int {
	return q.items.Len()
}
