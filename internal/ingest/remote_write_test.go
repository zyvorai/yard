package ingest

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/golang/snappy"
)

func TestRemoteWriteRoundTrip(t *testing.T) {
	when := time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)
	body := EncodeRemoteWrite([]Series{{
		Labels:  map[string]string{"__name__": "temperature", "asset": "ZY-SEN-0412"},
		Samples: []Sample{{Value: 21.5, Time: when.UnixMilli()}},
	}})
	got, err := DecodeRemoteWrite(body)
	if err != nil {
		t.Fatal(err)
	}
	obs := ObservationsFromRemoteWrite(got)
	if len(obs) != 1 || obs[0].AssetExternalRef != "ZY-SEN-0412" || obs[0].Capability != "temperature" || obs[0].Value != 21.5 {
		t.Fatalf("%+v", obs)
	}
	if !obs[0].ObservedAt.Equal(when) {
		t.Fatalf("time %s", obs[0].ObservedAt)
	}
}

func TestRemoteWriteHandBuilt(t *testing.T) {
	labelName := []byte{0x0a, 8, '_', '_', 'n', 'a', 'm', 'e', '_', '_', 0x12, 4, 't', 'e', 'm', 'p'}
	labelAsset := []byte{0x0a, 5, 'a', 's', 's', 'e', 't', 0x12, 2, 'Z', 'Y'}
	sample := []byte{0x09}
	var bits [8]byte
	binary.LittleEndian.PutUint64(bits[:], math.Float64bits(1))
	sample = append(sample, bits[:]...)
	sample = append(sample, 0x10, 0xe8, 0x07)
	series := append([]byte{0x0a, byte(len(labelName))}, labelName...)
	series = append(series, 0x0a, byte(len(labelAsset)))
	series = append(series, labelAsset...)
	series = append(series, 0x12, byte(len(sample)))
	series = append(series, sample...)
	req := append([]byte{0x0a, byte(len(series))}, series...)
	got, err := DecodeRemoteWrite(snappy.Encode(nil, req))
	if err != nil {
		t.Fatal(err)
	}
	obs := ObservationsFromRemoteWrite(got)
	if len(obs) != 1 || obs[0].Capability != "temp" || obs[0].AssetExternalRef != "ZY" || obs[0].Value != 1 {
		t.Fatalf("%+v", obs)
	}
}

func TestOTLPGauge(t *testing.T) {
	body := []byte(`{"resourceMetrics":[{"resource":{"attributes":[{"key":"yard.asset","value":{"stringValue":"ZY-SEN-0412"}}]},"scopeMetrics":[{"metrics":[{"name":"temperature","gauge":{"dataPoints":[{"asDouble":4.5,"timeUnixNano":"1758456000000000000"}]}}]}]}]}`)
	obs, err := ObservationsFromOTLP(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 || obs[0].AssetExternalRef != "ZY-SEN-0412" || obs[0].Value != 4.5 || obs[0].Capability != "temperature" {
		t.Fatalf("%+v", obs)
	}
}
