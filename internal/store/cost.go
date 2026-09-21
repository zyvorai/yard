package store

import (
	"context"
	"time"
)

type CostReport struct {
	EnergyCents      int64   `json:"energy_cents"`
	DowntimeCents    int64   `json:"downtime_cents"`
	RepairCents      int64   `json:"repair_cents"`
	ReplacementCents int64   `json:"replacement_cents"`
	CarbonGrams      float64 `json:"carbon_grams"`
	ForecastCents    int64   `json:"forecast_cents"`
}

func tariffCents(hour int, tariffs []Tariff, fallback int) int {
	for _, t := range tariffs {
		if t.StartHour == t.EndHour {
			continue
		}
		if t.StartHour < t.EndHour {
			if hour >= t.StartHour && hour < t.EndHour {
				return t.CentsPerKWh
			}
			continue
		}
		if hour >= t.StartHour || hour < t.EndHour {
			return t.CentsPerKWh
		}
	}
	return fallback
}

func (s *Store) CostReport(ctx context.Context, orgID string, now time.Time) (CostReport, error) {
	var out CostReport
	rate, err := s.EnergyCentsPerKWh(ctx, orgID)
	if err != nil {
		return out, err
	}
	tariffs, err := s.ListTariffs(ctx, orgID)
	if err != nil {
		return out, err
	}
	gramsPer, err := s.CarbonGrams(ctx, orgID)
	if err != nil {
		return out, err
	}
	type sample struct {
		at  time.Time
		kwh float64
	}
	var samples []sample
	obsRows, err := s.query(ctx, `SELECT observed_at, value FROM observations WHERE organization_id=? AND capability='energy_kwh' AND value_kind='number'`, orgID)
	if err != nil {
		return out, err
	}
	for obsRows.Next() {
		var at string
		var v float64
		if err := obsRows.Scan(&at, &v); err != nil {
			obsRows.Close()
			return out, err
		}
		samples = append(samples, sample{at: parseTime(at), kwh: v})
	}
	obsRows.Close()
	if err := obsRows.Err(); err != nil {
		return out, err
	}
	rollRows, err := s.query(ctx, `SELECT bucket_start, value_sum FROM observation_rollups WHERE organization_id=? AND capability='energy_kwh'`, orgID)
	if err != nil {
		return out, err
	}
	for rollRows.Next() {
		var at string
		var v float64
		if err := rollRows.Scan(&at, &v); err != nil {
			rollRows.Close()
			return out, err
		}
		samples = append(samples, sample{at: parseTime(at), kwh: v})
	}
	rollRows.Close()
	if err := rollRows.Err(); err != nil {
		return out, err
	}
	var kwh float64
	daily := map[string]float64{}
	for _, sample := range samples {
		kwh += sample.kwh
		cents := rate
		if len(tariffs) > 0 {
			cents = tariffCents(sample.at.UTC().Hour(), tariffs, rate)
		}
		out.EnergyCents += int64(sample.kwh * float64(cents))
		daily[sample.at.UTC().Format("2006-01-02")] += sample.kwh * float64(cents)
	}
	out.CarbonGrams = kwh * float64(gramsPer)
	if len(daily) > 0 {
		var sum float64
		n := 0
		for _, v := range daily {
			sum += v
			n++
			if n == 7 {
				break
			}
		}
		out.ForecastCents = int64(sum / float64(n) * 7)
	}
	var repair, replacement float64
	if err := s.queryRow(ctx, `SELECT COALESCE(SUM(quantity * unit_cost_cents),0) FROM work_order_lines WHERE organization_id=?`, orgID).Scan(&repair); err != nil {
		return out, err
	}
	if err := s.queryRow(ctx, `SELECT COALESCE(SUM(replacement_cost_cents),0) FROM assets WHERE organization_id=?`, orgID).Scan(&replacement); err != nil {
		return out, err
	}
	out.RepairCents = int64(repair)
	out.ReplacementCents = int64(replacement)
	incs, err := s.ListIncidents(ctx, orgID, "open")
	if err != nil {
		return out, err
	}
	for _, inc := range incs {
		if inc.AssetID == nil {
			continue
		}
		asset, err := s.AssetByID(ctx, orgID, *inc.AssetID)
		if err != nil || asset == nil || asset.DowntimeCentsPerHour <= 0 {
			continue
		}
		hours := now.Sub(inc.OpenedAt).Hours()
		if hours < 0 {
			hours = 0
		}
		out.DowntimeCents += int64(hours * float64(asset.DowntimeCentsPerHour))
	}
	return out, nil
}
