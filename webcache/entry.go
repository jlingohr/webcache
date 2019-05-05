package webcache

import (
	"container/list"
	"time"
)

type Response struct {
	ExpirationTime time.Time
	Body Value
	ContentType string
	//Size int
}

type Entry struct {
	Key            string
	//Value          Value
	*Response
	Size           int
	//ExpirationTime time.Time
	hits           uint64
	tick           uint64
	element        *list.Element
	index          int
}

func NewEntry(key string, response *Response) *Entry {
	return &Entry{
		Key:            key,
		//Value:          value,
		Response: response,
		Size:           len(response.Body),
		//ExpirationTime: ExpirationTime,
		hits:           0,
		tick:           0,
		index: -1,
	}
}

func (entry *Entry) Less(other *Entry) bool {
	if entry.hits < other.hits { return true }
	if entry.hits == other.hits { return entry.tick < other.tick
	}
	return false
}

func (entry *Entry) Expired() bool {
	return entry.ExpirationTime.Before(time.Now())
}

//func (entry *Entry) Update(duration time.Duration) {
//	entry.ExpirationTime = time.Now().Add(duration).UnixNano()
//}