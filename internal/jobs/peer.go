package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

func (e *Engine) ReplicateEvents(ctx context.Context) error {
	peer := os.Getenv("YARD_PEER_URL")
	if peer == "" {
		return nil
	}
	if e.Policy != nil {
		if err := e.Policy.Validate(peer); err != nil {
			return err
		}
	}
	list, err := e.Store.UnreplicatedEvents(ctx, os.Getenv("YARD_REGION"), 50)
	if err != nil || len(list) == 0 {
		return err
	}
	body, _ := json.Marshal(map[string]any{"events": list})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, peer, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := os.Getenv("YARD_PEER_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return errPeerStatus
	}
	ids := make([]string, 0, len(list))
	for _, ev := range list {
		ids = append(ids, ev.ID)
	}
	return e.Store.MarkEventsReplicated(ctx, ids)
}
