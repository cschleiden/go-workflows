package memory

type Queue interface {
	Push(v interface{})
	Pop() interface{}
}

type queue struct {
	s []interface{}
}

func NewQueue() Queue {
	return &queue{
		s: make([]interface{}, 0),
	}
}

func (q *queue) Push(v interface{}) {
	q.s = append(q.s, v)
}

func (q *queue) Pop() interface{} {
	if len(q.s) == 0 {
		return nil
	}

	v := q.s[0]

	q.s = q.s[1:]

	return v
}
