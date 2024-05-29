package core

import (
	"database/sql"
	"errors"
	"regexp"
)

type Queue string

var _ sql.Scanner = (*Queue)(nil)

func (q Queue) Value() (string, error) {
	return string(q), nil
}

func (q *Queue) Scan(value interface{}) error {
	*q = Queue(value.(string))
	return nil
}

const (
	QueueDefault = Queue("")
	QueueSystem  = Queue("_system_")
)

var validQueueName = regexp.MustCompile(`^[a-zA-Z0-9_-]{4,31}$`)

// ValidQueue ensures that the queue name is valid.
func ValidQueue(q Queue) error {
	if !validQueueName.MatchString(string(q)) {
		return errors.New("invalid queue name")
	}

	return nil
}
