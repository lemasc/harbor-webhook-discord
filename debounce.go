package main

import (
	"log"
	"sync"
	"time"
)

const debounceDuration = 5 * time.Second

type pendingGroup struct {
	event HarborEvent
	timer *time.Timer
}

type PushDebouncer struct {
	mu      sync.Mutex
	pending map[string]*pendingGroup
	send    func(HarborEvent) error
}

func NewPushDebouncer(send func(HarborEvent) error) *PushDebouncer {
	return &PushDebouncer{
		pending: make(map[string]*pendingGroup),
		send:    send,
	}
}

// Add buffers resources from event, grouped by (repository, digest).
// After debounceDuration of inactivity per group, the accumulated event is sent.
func (d *PushDebouncer) Add(event HarborEvent) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, res := range event.EventData.Resources {
		key := event.EventData.Repository.FullName + "@" + res.Digest
		g, ok := d.pending[key]
		if !ok {
			base := event
			base.EventData.Resources = []Resource{res}
			g = &pendingGroup{event: base}
			d.pending[key] = g
			localKey := key
			g.timer = time.AfterFunc(debounceDuration, func() {
				d.flush(localKey)
			})
		} else {
			g.event.EventData.Resources = append(g.event.EventData.Resources, res)
			g.timer.Reset(debounceDuration)
		}
	}
}

func (d *PushDebouncer) flush(key string) {
	d.mu.Lock()
	g, ok := d.pending[key]
	if !ok {
		d.mu.Unlock()
		return
	}
	delete(d.pending, key)
	event := g.event
	d.mu.Unlock()

	if err := d.send(event); err != nil {
		log.Printf("debouncer: send failed: %v", err)
	}
}
