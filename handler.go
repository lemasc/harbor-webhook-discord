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
	debouncer  *PushDebouncer
}

func NewWebhookHandler(discordURL string) *WebhookHandler {
	h := &WebhookHandler{discordURL: discordURL}
	h.debouncer = NewPushDebouncer(func(event HarborEvent) error {
		payload := buildPushPayload(event)
		return sendDiscordWebhook(h.discordURL, *payload)
	})
	return h
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

	switch event.Type {
	case EventPushArtifact:
		filtered := event.EventData.Resources[:0]
		for _, res := range event.EventData.Resources {
			if res.Tag != res.Digest {
				filtered = append(filtered, res)
			}
		}
		if len(filtered) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		event.EventData.Resources = filtered
		h.debouncer.Add(event)
		log.Printf("queued %s event for %s", event.Type, event.EventData.Repository.FullName)
		w.WriteHeader(http.StatusNoContent)
		return
	case EventQuotaExceed:
		payload := buildQuotaPayload(event)
		if err := sendDiscordWebhook(h.discordURL, *payload); err != nil {
			log.Printf("discord webhook: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		log.Printf("forwarded %s event for %s", event.Type, event.EventData.Repository.FullName)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func buildPushPayload(event HarborEvent) *DiscordPayload {
	timestamp := time.Unix(event.OccurAt, 0).UTC().Format(time.RFC3339)

	fields := []EmbedField{
		{Name: "Project", Value: event.EventData.Repository.Namespace, Inline: true},
		{Name: "Repository", Value: event.EventData.Repository.FullName, Inline: true},
		{Name: "Pushed by", Value: event.Operator, Inline: true},
	}

	// Group resources by digest so multiple tags for the same image collapse into one block.
	type group struct {
		tags   []string
		digest string
		url    string
	}
	seen := map[string]*group{}
	order := []string{}
	for _, res := range event.EventData.Resources {
		g, ok := seen[res.Digest]
		if !ok {
			g = &group{digest: res.Digest, url: res.ResourceURL}
			seen[res.Digest] = g
			order = append(order, res.Digest)
		}
		tag := res.Tag
		if tag == "" {
			tag = "(untagged)"
		}
		g.tags = append(g.tags, tag)
	}

	for _, digest := range order {
		g := seen[digest]
		tagValues := make([]string, len(g.tags))
		for i, t := range g.tags {
			tagValues[i] = fmt.Sprintf("`%s`", t)
		}
		tagLabel := "Tag"
		if len(g.tags) > 1 {
			tagLabel = "Tags"
		}
		fields = append(fields,
			EmbedField{Name: tagLabel, Value: strings.Join(tagValues, "\n"), Inline: true},
			EmbedField{Name: "Digest", Value: fmt.Sprintf("`%s`", shortDigest(digest)), Inline: true},
			EmbedField{Name: "Image", Value: g.url, Inline: false},
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
