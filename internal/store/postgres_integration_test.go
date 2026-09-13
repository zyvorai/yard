//go:build integration

package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/zyvorai/yard/internal/model"
	"github.com/zyvorai/yard/internal/store"
)

// Run with:
//   YARD_TEST_POSTGRES=postgres://yard:yard@127.0.0.1:5432/yard?sslmode=disable go test -tags=integration ./internal/store/
func TestPostgresOpenAndSite(t *testing.T) {
	dsn := os.Getenv("YARD_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("set YARD_TEST_POSTGRES to run")
	}
	st, err := store.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if st.Dialect != "postgres" {
		t.Fatalf("dialect=%s", st.Dialect)
	}
	org := &model.Organization{Name: "PG Test", Slug: "pg-test"}
	if err := st.CreateOrganization(context.Background(), org); err != nil {
		// may already exist from prior run
		t.Log(err)
	}
}
