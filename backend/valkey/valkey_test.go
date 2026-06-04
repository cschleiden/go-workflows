package valkey

import (
	"github.com/cschleiden/go-workflows/backend/test"
	"testing"
)

func Test_ValkeyBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	c := getClient()
	setup := getCreateBackend(c)

	test.BackendTest(t, setup, nil)
}

func Test_EndToEndValkeyBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	c := getClient()
	setup := getCreateBackend(c)

	test.EndToEndBackendTest(t, setup, nil)
}
