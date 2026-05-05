package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	EventPushArtifact = "PUSH_ARTIFACT"
	EventQuotaExceed  = "QUOTA_EXCEED"

	colorGreen = 0x57F287
	colorRed   = 0xED4245
)

type WebhookHandler struct {
	discordURL string
}

func NewWebhookHandler(discordURL string) *WebhookHandler {
	return &WebhookHandler{discordURL: discordURL}
}

func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("read body: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var event HarborEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("parse payload: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var payload *DiscordPayload
	switch event.Type {
	case EventPushArtifact:
		payload = buildPushPayload(event)
	case EventQuotaExceed:
		payload = buildQuotaPayload(event)
	default:
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := sendDiscordWebhook(h.discordURL, *payload); err != nil {
		log.Printf("discord webhook: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("forwarded %s event for %s", event.Type, event.EventData.Repository.FullName)
	w.WriteHeader(http.StatusNoContent)
}

func buildPushPayload(event HarborEvent) *DiscordPayload {
	timestamp := time.Unix(event.OccurAt, 0).UTC().Format(time.RFC3339)

	fields := []EmbedField{
		{Name: "Project", Value: event.EventData.Repository.Namespace, Inline: true},
		{Name: "Repository", Value: event.EventData.Repository.FullName, Inline: true},
		{Name: "Pushed by", Value: event.Operator, Inline: true},
	}

	for _, res := range event.EventData.Resources {
		tag := res.Tag
		if tag == "" {
			tag = "(untagged)"
		}
		fields = append(fields,
			EmbedField{Name: "Tag", Value: fmt.Sprintf("`%s`", tag), Inline: true},
			EmbedField{Name: "Digest", Value: fmt.Sprintf("`%s`", shortDigest(res.Digest)), Inline: true},
			EmbedField{Name: "Image", Value: res.ResourceURL, Inline: false},
		)
	}

	return &DiscordPayload{
		Embeds: []DiscordEmbed{{
			Title:     "Image Pushed",
			Color:     colorGreen,
			Fields:    fields,
			Timestamp: timestamp,
			Footer:    &EmbedFooter{Text: "Harbor Registry"},
		}},
	}
}

func buildQuotaPayload(event HarborEvent) *DiscordPayload {
	timestamp := time.Unix(event.OccurAt, 0).UTC().Format(time.RFC3339)

	return &DiscordPayload{
		Embeds: []DiscordEmbed{{
			Title:       "Project Quota Exceeded",
			Description: fmt.Sprintf("Project **%s** has exceeded its storage quota.", event.EventData.Repository.Namespace),
			Color:       colorRed,
			Fields: []EmbedField{
				{Name: "Project", Value: event.EventData.Repository.Namespace, Inline: true},
				{Name: "Repository", Value: event.EventData.Repository.FullName, Inline: true},
				{Name: "Triggered by", Value: event.Operator, Inline: true},
			},
			Timestamp: timestamp,
			Footer:    &EmbedFooter{Text: "Harbor Registry"},
		}},
	}
}

func shortDigest(digest string) string {
	const prefix = "sha256:"
	if strings.HasPrefix(digest, prefix) && len(digest) > len(prefix)+12 {
		return prefix + digest[len(prefix):len(prefix)+12] + "..."
	}
	return digest
}
