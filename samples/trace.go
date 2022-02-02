package samples

import (
	"fmt"
	"log"
	"time"

	"github.com/cschleiden/go-dt/pkg/workflow"
)

func Trace(ctx workflow.Context, v ...interface{}) {
	var r string
	if workflow.Replaying(ctx) {
		r = "[replay]"
	}

	prefix := fmt.Sprintf("[%v][%v]%v", workflow.WorkflowInstance(ctx).GetInstanceID(), workflow.Now(ctx).Format(time.StampMilli), r)
	args := make([]interface{}, len(v)+1)
	args[0] = prefix
	copy(args[1:], v)
	log.Println(args...)
}
