package valkey

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/backend/history"
	"github.com/cschleiden/go-workflows/backend/test"
	"github.com/cschleiden/go-workflows/client"
	"github.com/cschleiden/go-workflows/diag"
	"github.com/stretchr/testify/require"
	"github.com/valkey-io/valkey-go"
)

func getClient() valkey.Client {
	newClient, _ := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{"localhost:6379"},
		Password:    "ValkeyPassw0rd",
		SelectDB:    0,
	})
	return newClient
}

func getCreateBackend(client valkey.Client, additionalOptions ...BackendOption) func(options ...backend.BackendOption) test.TestBackend {
	return func(options ...backend.BackendOption) test.TestBackend {
		// Flush database
		if err := client.Do(context.Background(), client.B().Flushdb().Build()).Error(); err != nil {
			panic(err)
		}

		r, err := client.Do(context.Background(), client.B().Keys().Pattern("*").Build()).AsStrSlice()
		if err != nil {
			panic(err)
		}

		if len(r) > 0 {
			panic("Keys should've been empty" + strings.Join(r, ", "))
		}

		redisOptions := []BackendOption{
			WithBlockTimeout(time.Millisecond * 10),
			WithBackendOptions(options...),
		}

		redisOptions = append(redisOptions, additionalOptions...)

		b, err := NewValkeyBackend(client, redisOptions...)
		if err != nil {
			panic(err)
		}

		return b
	}
}

var _ test.TestBackend = (*valkeyBackend)(nil)

// GetFutureEvents
func (vb *valkeyBackend) GetFutureEvents(ctx context.Context) ([]*history.Event, error) {
	r, err := vb.client.Do(ctx, vb.client.B().Zrangebyscore().Key(vb.keys.futureEventsKey()).Min("-inf").Max("+inf").Build()).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("getting future events: %w", err)
	}

	events := make([]*history.Event, 0)

	for _, eventID := range r {
		eventStr, err := vb.client.Do(ctx, vb.client.B().Hget().Key(eventID).Field("event").Build()).AsBytes()
		if err != nil {
			return nil, fmt.Errorf("getting event %v: %w", eventID, err)
		}

		var event *history.Event
		if err := json.Unmarshal(eventStr, &event); err != nil {
			return nil, fmt.Errorf("unmarshaling event %v: %w", eventID, err)
		}

		events = append(events, event)
	}

	return events, nil
}

func Test_Diag_GetWorkflowInstances(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	c := getClient()
	t.Cleanup(func() { c.Close() })

	vc := getCreateBackend(c)()

	bd := vc.(diag.Backend)

	ctx := context.Background()
	instances, err := bd.GetWorkflowInstances(ctx, "", "", 5)
	require.NoError(t, err)
	require.Empty(t, instances)

	cl := client.New(bd)

	_, err = cl.CreateWorkflowInstance(ctx, client.WorkflowInstanceOptions{
		InstanceID: "ex1",
	}, "some-workflow")
	require.NoError(t, err)

	instances, err = bd.GetWorkflowInstances(ctx, "", "", 5)
	require.NoError(t, err)
	require.Len(t, instances, 1)
	require.Equal(t, "ex1", instances[0].Instance.InstanceID)
}
