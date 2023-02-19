package redis

import (
	"fmt"
	"strconv"
	"strings"
)

func instanceKey(instanceID string) string {
	return fmt.Sprintf("instance:%v", instanceID)
}

func instancesByCreation() string {
	return "instances-by-creation"
}

func pendingEventsKey(instanceID string) string {
	return fmt.Sprintf("pending-events:%v", instanceID)
}

func historyKey(instanceID string) string {
	return fmt.Sprintf("history:%v", instanceID)
}

func historyID(sequenceID int64) string {
	return fmt.Sprintf("%v-0", sequenceID)
}

func sequenceIdFromHistoryID(historyID string) (int64, error) {
	idStr := strings.Split(historyID, "-")[0]
	return strconv.ParseInt(idStr, 10, 64)
}

func futureEventsKey() string {
	return "future-events"
}

func futureEventKey(instanceID string, scheduleEventID int64) string {
	return fmt.Sprintf("future-event:%v:%v", instanceID, scheduleEventID)
}

func eventKey(eventID string) string {
	return fmt.Sprintf("event:%v", eventID)
}
