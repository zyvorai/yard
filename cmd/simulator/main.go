package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

type obs struct {
	AssetExternalRef string    `json:"asset_external_ref"`
	Capability       string    `json:"capability"`
	Value            float64   `json:"value"`
	Unit             string    `json:"unit"`
	Quality          string    `json:"quality"`
	Source           string    `json:"source"`
	ObservedAt       time.Time `json:"observed_at"`
	DedupeKey        string    `json:"dedupe_key"`
}

func main() {
	base := env("ESTATE_URL", "http://127.0.0.1:8080")
	tok := strings.TrimSpace(env("ESTATE_SIMULATOR_TOKEN", readFile(env("ESTATE_SIMULATOR_TOKEN_FILE", "data/simulator.token"))))
	if tok == "" {
		log.Fatal("ESTATE_SIMULATOR_TOKEN is required")
	}
	trip := env("ESTATE_SIM_TRIP", "auto")
	log.Printf("simulator publishing to %s", base)
	t := 0
	for {
		now := time.Now().UTC()
		batch := []obs{
			pt("ZY-GW-0001", "cpu_temp", 42+3*math.Sin(float64(t)/8), "°C", now, t),
			pt("ZY-GW-0001", "heartbeat", 1, "1", now, t),
			pt("ZY-SEN-0412", "temperature", 3.2+0.4*math.Sin(float64(t)/6), "°C", now, t),
			pt("ZY-SEN-0412", "humidity", 78+2*math.Sin(float64(t)/9), "%", now, t),
			pt("ZY-PUMP-03", "temperature", 61+4*math.Sin(float64(t)/10), "°C", now, t),
			pt("ZY-PUMP-03", "vibration", 1.4+0.2*rand.Float64(), "mm/s", now, t),
			pt("ZY-PUMP-03", "heartbeat", 1, "1", now, t),
			pt("ZY-CNC-08", "spindle_load", 35+20*rand.Float64(), "%", now, t),
			pt("ZY-CNC-08", "heartbeat", 1, "1", now, t),
			pt("ZY-VEH-17", "speed", 20+15*rand.Float64(), "km/h", now, t),
			pt("ZY-VEH-17", "fuel", 62, "%", now, t),
			pt("ZY-VEH-17", "heartbeat", 1, "1", now, t),
			pt("ZY-SEN-D2", "open", openVal(t), "1", now, t),
			pt("ZY-SEN-D2", "heartbeat", 1, "1", now, t),
		}
		temp := 48 + 8*math.Sin(float64(t)/12)
		if trip == "auto" && t > 8 && t%20 < 4 {
			temp = 82 + rand.Float64()*6
		}
		if trip == "hot" {
			temp = 86
		}
		batch = append(batch, pt("SIM-TEMP-A", "temperature", temp, "°C", now, t))
		batch = append(batch, pt("SIM-TEMP-A", "heartbeat", 1, "1", now, t))
		body, _ := json.Marshal(batch)
		req, _ := http.NewRequest(http.MethodPost, strings.TrimRight(base, "/")+"/api/v1/ingest/observations", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("publish error: %v", err)
		} else {
			resp.Body.Close()
			log.Printf("tick=%d status=%d temp=%.1f", t, resp.StatusCode, temp)
		}
		t++
		time.Sleep(5 * time.Second)
	}
}

func pt(ref, cap string, v float64, unit string, now time.Time, tick int) obs {
	return obs{
		AssetExternalRef: ref,
		Capability:       cap,
		Value:            v,
		Unit:             unit,
		Quality:          "good",
		Source:           "simulator",
		ObservedAt:       now,
		DedupeKey:        ref + ":" + cap + ":" + now.Format("20060102150405"),
	}
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func readFile(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

func openVal(t int) float64 {
	if t%11 == 0 {
		return 1
	}
	return 0
}
