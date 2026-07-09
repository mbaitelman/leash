package action

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mbaitelman/leash/internal/resource"
)

func init() {
	Register("notify", newNotifyAction)
}

// notifyClient bounds webhook delivery so a hung endpoint can't stall a run.
var notifyClient = &http.Client{Timeout: 15 * time.Second}

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
	props := r.Properties()
	name, _ := props["name"].(string)
	if name == "" {
		name, _ = props["title"].(string)
	}
	if name == "" {
		name, _ = props["email"].(string)
	}

	summary := fmt.Sprintf("*%s* · `%s`", r.Type(), r.ID())
	if name != "" {
		summary = fmt.Sprintf("*%s* · `%s`\n%s", r.Type(), r.ID(), name)
	}

	if dryRun {
		slog.Info("notify (dry-run)", "resource_type", r.Type(), "resource_id", r.ID(), "name", name)
		return nil
	}

	if a.webhookURL == "" {
		return fmt.Errorf("notify action: no webhook_url configured and SLACK_WEBHOOK_URL is not set")
	}

	body, err := json.Marshal(buildSlackPayload(r.Type(), r.ID(), summary, props))
	if err != nil {
		return fmt.Errorf("notify action: marshaling payload: %w", err)
	}

	resp, err := notifyClient.Post(a.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify action: sending to Slack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("notify action: Slack returned HTTP %d", resp.StatusCode)
	}
	return nil
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

type slackAttachment struct {
	Color    string       `json:"color"`
	Fallback string       `json:"fallback"`
	Title    string       `json:"title"`
	Text     string       `json:"text"`
	Fields   []slackField `json:"fields,omitempty"`
	Footer   string       `json:"footer"`
	Ts       int64        `json:"ts"`
}

type slackPayload struct {
	Attachments []slackAttachment `json:"attachments"`
}

func buildSlackPayload(resType, resID, summary string, props map[string]any) slackPayload {
	fields := []slackField{
		{Title: "Resource", Value: fmt.Sprintf("%s · %s", resType, resID), Short: true},
	}

	if status := stringProp(props, "status", "overall_state"); status != "" {
		fields = append(fields, slackField{Title: "Status", Value: status, Short: true})
	}

	if tags := tagsProp(props); tags != "" {
		fields = append(fields, slackField{Title: "Tags", Value: tags, Short: false})
	}

	return slackPayload{
		Attachments: []slackAttachment{
			{
				Color:    "warning",
				Fallback: fmt.Sprintf("[leash] Policy violation: %s %s", resType, resID),
				Title:    ":warning:  Leash policy violation",
				Text:     summary,
				Fields:   fields,
				Footer:   "Leash",
				Ts:       time.Now().Unix(),
			},
		},
	}
}

// stringProp returns the first non-empty string value found among the given keys.
func stringProp(props map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, _ := props[k].(string); v != "" {
			return v
		}
	}
	return ""
}

// tagsProp formats the tags list as a comma-separated string.
func tagsProp(props map[string]any) string {
	switch v := props["tags"].(type) {
	case []string:
		return strings.Join(v, ", ")
	case []any:
		parts := make([]string, 0, len(v))
		for _, t := range v {
			parts = append(parts, fmt.Sprintf("%v", t))
		}
		return strings.Join(parts, ", ")
	}
	return ""
}
