package store

import "testing"

func TestParseDSN(t *testing.T) {
	cases := []struct {
		in, driver, dialect string
	}{
		{"file:data/yard.db", "sqlite", "sqlite"},
		{"sqlite://file:x.db", "sqlite", "sqlite"},
		{"postgres://u:p@localhost:5432/yard?sslmode=disable", "pgx", "postgres"},
		{"postgresql://localhost/yard", "pgx", "postgres"},
	}
	for _, c := range cases {
		d, di, _ := parseDSN(c.in)
		if d != c.driver || di != c.dialect {
			t.Fatalf("%s → %s/%s want %s/%s", c.in, d, di, c.driver, c.dialect)
		}
	}
}

func TestRebindPostgres(t *testing.T) {
	s := &Store{Dialect: "postgres"}
	got := s.rebind("SELECT a FROM t WHERE x=? AND y=?")
	want := "SELECT a FROM t WHERE x=$1 AND y=$2"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	s.Dialect = "sqlite"
	if s.rebind("SELECT ?") != "SELECT ?" {
		t.Fatal("sqlite should not rebind")
	}
}
