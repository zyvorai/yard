package ingest

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/golang/snappy"
	"github.com/zyvorai/yard/internal/model"
)

// Series is one Prometheus remote-write time series.
type Series struct {
	Labels  map[string]string
	Samples []Sample
}

// Sample is one remote-write point. Time is milliseconds since epoch.
type Sample struct {
	Value float64
	Time  int64
}

// DecodeRemoteWrite reads a snappy-compressed Prometheus WriteRequest.
func DecodeRemoteWrite(body []byte) ([]Series, error) {
	raw, err := snappy.Decode(nil, body)
	if err != nil {
		return nil, fmt.Errorf("snappy: %w", err)
	}
	return decodeWriteRequest(raw)
}

func decodeWriteRequest(b []byte) ([]Series, error) {
	var out []Series
	for len(b) > 0 {
		field, wt, rest, err := readKey(b)
		if err != nil {
			return nil, err
		}
		b = b[rest:]
		if field == 1 && wt == 2 {
			msg, n, err := readBytes(b)
			if err != nil {
				return nil, err
			}
			s, err := decodeSeries(msg)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
			b = b[n:]
			continue
		}
		b, err = skip(b, wt)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func decodeSeries(b []byte) (Series, error) {
	s := Series{Labels: map[string]string{}}
	for len(b) > 0 {
		field, wt, rest, err := readKey(b)
		if err != nil {
			return s, err
		}
		b = b[rest:]
		switch {
		case field == 1 && wt == 2:
			msg, n, err := readBytes(b)
			if err != nil {
				return s, err
			}
			name, value, err := decodeLabel(msg)
			if err != nil {
				return s, err
			}
			s.Labels[name] = value
			b = b[n:]
		case field == 2 && wt == 2:
			msg, n, err := readBytes(b)
			if err != nil {
				return s, err
			}
			sample, err := decodeSample(msg)
			if err != nil {
				return s, err
			}
			s.Samples = append(s.Samples, sample)
			b = b[n:]
		default:
			b, err = skip(b, wt)
			if err != nil {
				return s, err
			}
		}
	}
	return s, nil
}

func decodeLabel(b []byte) (string, string, error) {
	var name, value string
	for len(b) > 0 {
		field, wt, rest, err := readKey(b)
		if err != nil {
			return "", "", err
		}
		b = b[rest:]
		if wt == 2 && (field == 1 || field == 2) {
			msg, n, err := readBytes(b)
			if err != nil {
				return "", "", err
			}
			if field == 1 {
				name = string(msg)
			} else {
				value = string(msg)
			}
			b = b[n:]
			continue
		}
		b, err = skip(b, wt)
		if err != nil {
			return "", "", err
		}
	}
	return name, value, nil
}

func decodeSample(b []byte) (Sample, error) {
	var s Sample
	for len(b) > 0 {
		field, wt, rest, err := readKey(b)
		if err != nil {
			return s, err
		}
		b = b[rest:]
		switch {
		case field == 1 && wt == 1:
			if len(b) < 8 {
				return s, fmt.Errorf("short sample value")
			}
			s.Value = math.Float64frombits(binary.LittleEndian.Uint64(b[:8]))
			b = b[8:]
		case field == 2 && wt == 0:
			v, n, err := readVarint(b)
			if err != nil {
				return s, err
			}
			s.Time = int64(v)
			b = b[n:]
		default:
			b, err = skip(b, wt)
			if err != nil {
				return s, err
			}
		}
	}
	return s, nil
}

func readKey(b []byte) (field, wt, n int, err error) {
	v, n, err := readVarint(b)
	if err != nil {
		return 0, 0, 0, err
	}
	return int(v >> 3), int(v & 7), n, nil
}

func readVarint(b []byte) (uint64, int, error) {
	var x uint64
	for i := 0; i < len(b) && i < 10; i++ {
		c := b[i]
		x |= uint64(c&0x7f) << (7 * i)
		if c < 0x80 {
			return x, i + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("bad varint")
}

func readBytes(b []byte) ([]byte, int, error) {
	n, ln, err := readVarint(b)
	if err != nil {
		return nil, 0, err
	}
	if uint64(len(b)-ln) < n {
		return nil, 0, fmt.Errorf("short bytes")
	}
	end := ln + int(n)
	return b[ln:end], end, nil
}

func skip(b []byte, wt int) ([]byte, error) {
	switch wt {
	case 0:
		_, n, err := readVarint(b)
		if err != nil {
			return nil, err
		}
		return b[n:], nil
	case 1:
		if len(b) < 8 {
			return nil, fmt.Errorf("short fixed64")
		}
		return b[8:], nil
	case 2:
		_, n, err := readBytes(b)
		if err != nil {
			return nil, err
		}
		return b[n:], nil
	case 5:
		if len(b) < 4 {
			return nil, fmt.Errorf("short fixed32")
		}
		return b[4:], nil
	default:
		return nil, fmt.Errorf("wire type %d", wt)
	}
}

// EncodeRemoteWrite builds a snappy WriteRequest for tests and small bridges.
func EncodeRemoteWrite(series []Series) []byte {
	var raw []byte
	for _, s := range series {
		raw = appendBytes(raw, 1, encodeSeries(s))
	}
	return snappy.Encode(nil, raw)
}

func encodeSeries(s Series) []byte {
	var raw []byte
	for k, v := range s.Labels {
		raw = appendBytes(raw, 1, encodeLabel(k, v))
	}
	for _, sample := range s.Samples {
		raw = appendBytes(raw, 2, encodeSample(sample))
	}
	return raw
}

func encodeLabel(name, value string) []byte {
	var raw []byte
	raw = appendBytes(raw, 1, []byte(name))
	raw = appendBytes(raw, 2, []byte(value))
	return raw
}

func encodeSample(s Sample) []byte {
	var raw []byte
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], math.Float64bits(s.Value))
	raw = append(raw, byte(1<<3|1))
	raw = append(raw, buf[:]...)
	raw = appendVarintField(raw, 2, uint64(s.Time))
	return raw
}

func appendBytes(dst []byte, field int, msg []byte) []byte {
	dst = appendVarint(dst, uint64(field<<3|2))
	dst = appendVarint(dst, uint64(len(msg)))
	return append(dst, msg...)
}

func appendVarintField(dst []byte, field int, v uint64) []byte {
	dst = appendVarint(dst, uint64(field<<3|0))
	return appendVarint(dst, v)
}

func appendVarint(dst []byte, v uint64) []byte {
	for v >= 0x80 {
		dst = append(dst, byte(v)|0x80)
		v >>= 7
	}
	return append(dst, byte(v))
}

// ObservationsFromRemoteWrite maps series that carry an asset label.
func ObservationsFromRemoteWrite(series []Series) []model.IngestObservation {
	var out []model.IngestObservation
	for _, s := range series {
		ref := firstLabel(s.Labels, "asset", "yard_asset", "external_ref")
		name := s.Labels["__name__"]
		if ref == "" || name == "" {
			continue
		}
		for _, sample := range s.Samples {
			out = append(out, model.IngestObservation{
				AssetExternalRef: ref,
				Capability:       name,
				Value:            sample.Value,
				ObservedAt:       time.UnixMilli(sample.Time).UTC(),
				Source:           "remote-write",
			})
		}
	}
	return out
}

func firstLabel(labels map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := labels[k]; v != "" {
			return v
		}
	}
	return ""
}

type otlpAttr struct {
	Key   string `json:"key"`
	Value struct {
		StringValue string `json:"stringValue"`
	} `json:"value"`
}

type otlpPoint struct {
	AsDouble     *float64        `json:"asDouble"`
	AsInt        json.RawMessage `json:"asInt"`
	TimeUnixNano json.RawMessage `json:"timeUnixNano"`
	Attributes   []otlpAttr      `json:"attributes"`
}

type otlpMetric struct {
	Name  string `json:"name"`
	Gauge *struct {
		DataPoints []otlpPoint `json:"dataPoints"`
	} `json:"gauge"`
	Sum *struct {
		DataPoints []otlpPoint `json:"dataPoints"`
	} `json:"sum"`
}

type otlpPayload struct {
	ResourceMetrics []struct {
		Resource struct {
			Attributes []otlpAttr `json:"attributes"`
		} `json:"resource"`
		ScopeMetrics []struct {
			Metrics []otlpMetric `json:"metrics"`
		} `json:"scopeMetrics"`
	} `json:"resourceMetrics"`
}

// ObservationsFromOTLP reads OTLP JSON metrics. Gauges and sums become numeric
// observations. Histograms are ignored.
func ObservationsFromOTLP(body []byte) ([]model.IngestObservation, error) {
	var doc otlpPayload
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	var out []model.IngestObservation
	for _, rm := range doc.ResourceMetrics {
		ref := attrString(rm.Resource.Attributes, "yard.asset", "service.name")
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				points := []otlpPoint{}
				if m.Gauge != nil {
					points = append(points, m.Gauge.DataPoints...)
				}
				if m.Sum != nil {
					points = append(points, m.Sum.DataPoints...)
				}
				for _, p := range points {
					asset := ref
					if v := attrString(p.Attributes, "yard.asset", "service.name"); v != "" {
						asset = v
					}
					if asset == "" || m.Name == "" {
						continue
					}
					val, ok := pointValue(p)
					if !ok {
						continue
					}
					out = append(out, model.IngestObservation{
						AssetExternalRef: asset,
						Capability:       m.Name,
						Value:            val,
						ObservedAt:       nanoTime(p.TimeUnixNano),
						Source:           "otlp",
					})
				}
			}
		}
	}
	return out, nil
}

func attrString(attrs []otlpAttr, keys ...string) string {
	for _, key := range keys {
		for _, a := range attrs {
			if a.Key == key && a.Value.StringValue != "" {
				return a.Value.StringValue
			}
		}
	}
	return ""
}

func pointValue(p otlpPoint) (float64, bool) {
	if p.AsDouble != nil {
		return *p.AsDouble, true
	}
	if len(p.AsInt) == 0 {
		return 0, false
	}
	raw := string(p.AsInt)
	if raw[0] == '"' {
		raw = raw[1 : len(raw)-1]
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func nanoTime(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Time{}
	}
	s := string(raw)
	if s[0] == '"' && len(s) >= 2 {
		s = s[1 : len(s)-1]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	return time.Unix(0, n).UTC()
}
