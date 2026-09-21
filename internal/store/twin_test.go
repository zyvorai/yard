package store

import (
	"context"
	"testing"

	"github.com/zyvorai/yard/internal/model"
)

func TestBlastRadiusDepth(t *testing.T) {
	st, err := Open("file:blast?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	org, err := st.CreateOrganization(context.Background(), "Ops", "ops-blast")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"a", "b", "c", "d"}
	for _, id := range ids {
		if err := st.UpsertAsset(context.Background(), &model.Asset{ID: id, OrganizationID: org.ID, Name: id, Kind: "equipment"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, link := range [][2]string{{"a", "b"}, {"b", "c"}, {"c", "d"}} {
		if err := st.CreateAssetLink(context.Background(), &AssetLink{OrganizationID: org.ID, FromAssetID: link[0], ToAssetID: link[1], Relation: "depends-on"}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.BlastRadius(context.Background(), org.ID, "a", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("radius %d", len(got))
	}
}
