package action

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/mbaitelman/leash/internal/resource"
)

func init() {
	Register("notify", newNotifyAction)
}

type notifyAction struct {
	webhookURL string
	channel    string
}

func newNotifyAction(spec map[string]any) (Action, error) {
	url, _ := spec["webhook_url"].(string)
	if url == "" {
		url = os.Getenv("SLACK_WEBHOOK_URL")
	}
	channel, _ := spec["channel"].(string)
	return &notifyAction{webhookURL: url, channel: channel}, nil
}

func (a *notifyAction) Type() string { return "notify" }

func (a *notifyAction) Execute(_ context.Context, r resource.Resource, dryRun bool) error {
	msg := fmt.Sprintf("[leash] Policy match: %s  resource=%s  id=%s", r.Type(), r.Type(), r.ID())

	if dryRun {
		slog.Info("notify (dry-run)", "message", msg, "channel", a.channel)
		return nil
	}

	if a.webhookURL == "" {
		return fmt.Errorf("notify action: no webhook_url configured and SLACK_WEBHOOK_URL is not set")
	}

	payload := map[string]string{"text": msg}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notify action: marshaling payload: %w", err)
	}

	resp, err := http.Post(a.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify action: sending to Slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notify action: Slack returned HTTP %d", resp.StatusCode)
	}
	return nil
}
