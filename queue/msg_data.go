package queue

import "time"

type Msg struct {
	ID     string         `json:"id"`
	Value  any            `json:"value"`
	Time   time.Time      `json:"time"`
	Status MsgQueueStatus `json:"status"`
}

type MsgQueueStatus string

const (
	ActiveStatus     MsgQueueStatus = "ACTIVE"
	ProcessingStatus MsgQueueStatus = "PROCESSING"
	AckedStatus      MsgQueueStatus = "ACKED"
	FailedStatus     MsgQueueStatus = "FAILED"
	ArchivedStatus   MsgQueueStatus = "ARCHIVED"
)

func (m MsgQueueStatus) String() string {
	return string(m)
}
