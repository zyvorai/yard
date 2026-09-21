package store

import (
	"testing"
	"time"

	"github.com/zyvorai/yard/internal/model"
)

func TestTariffCarbonAndForecast(t *testing.T) {
	st, err := Open("file:tariff?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	org, err := st.CreateOrganization(t.Context(), "Ops", "ops-tariff")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetEnergyCentsPerKWh(t.Context(), org.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceTariffs(t.Context(), org.ID, []Tariff{{StartHour: 0, EndHour: 24, CentsPerKWh: 20}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SetCarbonGrams(t.Context(), org.ID, 400); err != nil {
		t.Fatal(err)
	}
	asset := &model.Asset{OrganizationID: org.ID, Name: "Meter", Kind: "equipment"}
	if err := st.UpsertAsset(t.Context(), asset); err != nil {
		t.Fatal(err)
	}
	if _, err := st.InsertObservation(t.Context(), &model.Observation{
		OrganizationID: org.ID, AssetID: asset.ID, Capability: "energy_kwh", Value: 5, ValueKind: "number", ObservedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	report, err := st.CostReport(t.Context(), org.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if report.EnergyCents != 100 || report.CarbonGrams != 2000 || report.ForecastCents != 700 {
		t.Fatalf("%+v", report)
	}
}
