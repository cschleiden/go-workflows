package samples

import (
	"fmt"
	"log"

	"github.com/cschleiden/go-dt/pkg/workflow"
)

func Trace(ctx workflow.Context, v ...interface{}) {
	suffix := fmt.Sprintf("[replay: %v]", workflow.Replaying(ctx))

	v = append(v, suffix)
	log.Println(v...)
}
