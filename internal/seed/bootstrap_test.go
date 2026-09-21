package seed

import (
	"context"
	"testing"

	"github.com/zyvorai/yard/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func TestProductionBootstrapRefusesDemoData(t *testing.T) {
	st, err := store.Open("file:prod-boot?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if _, err := BootstrapWith(ctx, st, Options{Mode: "production"}); err == nil {
		t.Fatal("expected missing bootstrap env to fail")
	}
	if _, err := BootstrapWith(ctx, st, Options{Mode: "production", BootstrapEmail: DemoEmail, BootstrapPassword: "not-the-demo-pw", PublicURL: "https://yard.example"}); err == nil {
		t.Fatal("expected demo email to be rejected")
	}
	if _, err := BootstrapWith(ctx, st, Options{Mode: "production", BootstrapEmail: "ops@example.com", BootstrapPassword: DemoPassword, PublicURL: "https://yard.example"}); err == nil {
		t.Fatal("expected demo password to be rejected")
	}
	res, err := BootstrapWith(ctx, st, Options{Mode: "production", BootstrapEmail: "ops@example.com", BootstrapPassword: "correct-horse", PublicURL: "https://yard.example"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UserByEmail(ctx, DemoEmail); err == nil {
		t.Fatal("production database must not contain the demo user")
	}
	u, err := st.UserByEmail(ctx, "ops@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(DemoPassword)) == nil {
		t.Fatal("demo password must not authenticate")
	}
	assets, err := st.ListAssets(ctx, res.Organization.ID, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 0 {
		t.Fatalf("production bootstrap inserted %d sample assets", len(assets))
	}
}

func TestProductionRefusesExistingDemoPassword(t *testing.T) {
	st, err := store.Open("file:prod-existing?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	if _, err := Bootstrap(ctx, st); err != nil {
		t.Fatal(err)
	}
	_, err = BootstrapWith(ctx, st, Options{Mode: "production", PublicURL: "https://yard.example"})
	if err == nil {
		t.Fatal("production mode must refuse a database that still accepts the demo password")
	}
}
