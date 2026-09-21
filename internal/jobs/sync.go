package jobs

import (
	"context"
	"io"
	"net/http"

	"github.com/zyvorai/yard/internal/playbook"
)

// SyncPlaybooks refetches each playbook that has a source URL.
// A failed fetch leaves the stored body in place.
func (e *Engine) SyncPlaybooks(ctx context.Context) error {
	list, err := e.Store.ListSourcedPlaybooks(ctx)
	if err != nil {
		return err
	}
	for _, pb := range list {
		body, err := e.fetchText(ctx, pb.SourceURL)
		if err != nil {
			if e.Log != nil {
				e.Log.Error("playbook fetch", "id", pb.ID, "err", err)
			}
			continue
		}
		doc, err := playbook.Parse(body)
		if err != nil {
			continue
		}
		if body == pb.Body {
			continue
		}
		if err := e.Store.UpdatePlaybookBody(ctx, pb.OrganizationID, pb.ID, doc.Name, body); err != nil && e.Log != nil {
			e.Log.Error("playbook update", "id", pb.ID, "err", err)
		}
	}
	return nil
}

func (e *Engine) fetchText(ctx context.Context, rawURL string) (string, error) {
	if e.Policy == nil {
		return "", errNoPolicy
	}
	if err := e.Policy.Validate(rawURL); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := e.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", errFetch
	}
	buf, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return "", err
	}
	return string(buf), nil
}
