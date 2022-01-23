package mysql

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/cschleiden/go-dt/pkg/backend"
	"github.com/cschleiden/go-dt/pkg/backend/test"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

const testUser = "root"
const testPassword = "SqlPassw0rd"

// Creating and dropping databases is terribly inefficient, but easiest for complete test isolation. For
// the future consider nested transactions, or manually TRUNCATE-ing the tables in-between tests.

func Test_MysqlBackend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	dbName := "test_" + strings.Replace(uuid.NewString(), "-", "", -1)

	test.TestBackend(t, test.Tester{
		New: func() backend.Backend {
			db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/?parseTime=true&interpolateParams=true", testUser, testPassword))
			if err != nil {
				panic(err)
			}

			if _, err := db.Exec("CREATE DATABASE " + dbName); err != nil {
				panic(errors.Wrap(err, "could not create database"))
			}

			if err := db.Close(); err != nil {
				panic(err)
			}

			return NewMysqlBackend(testUser, testPassword, dbName, backend.WithStickyTimeout(0))
		},

		Teardown: func() {
			db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/?parseTime=true&interpolateParams=true", testUser, testPassword))
			if err != nil {
				panic(err)
			}

			if _, err := db.Exec("DROP DATABASE IF EXISTS " + dbName); err != nil {
				panic(errors.Wrap(err, "could not drop database"))
			}

			if err := db.Close(); err != nil {
				panic(err)
			}
		},
	})
}
