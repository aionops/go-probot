package probot

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/aionops/go-probot/web/backend"
)

type eventMetrics struct {
	events sync.Map
}

var _ backend.EventSource = &eventMetrics{}

func (m *eventMetrics) add(name, tp string) {
	m.events.Store(name, backend.Event{
		Name: name, Type: backend.EventType(tp),
		Metrics: backend.EventMetrics{},
	})
}

func (m *eventMetrics) inc(name, tp string) {
	e, _ := m.events.LoadOrStore(name, backend.Event{
		Name: name, Type: backend.EventType(tp),
		Metrics: backend.EventMetrics{},
	})
	event := e.(backend.Event)
	event.Metrics.Count++
	m.events.Store(name, event)
}

func (m *eventMetrics) ListEvent(ctx context.Context, opts backend.ListOptions) (backend.EventList, error) {
	out := backend.EventList{}
	m.events.Range(func(key, value any) bool {
		out.Items = append(out.Items, value.(backend.Event))
		return true
	})
	sort.Slice(out.Items, func(i, j int) bool {
		return strings.Compare(out.Items[i].Name, out.Items[j].Name) < 0
	})
	if opts.Limit > 0 {
		start := opts.Offset * opts.Limit
		end := opts.Offset*opts.Limit + opts.Limit
		if start < 0 {
			start = 0
		} else if start > int64(len(out.Items)) {
			start = int64(len(out.Items))
		}
		if end < 0 {
			end = 0
		} else if end > int64(len(out.Items)) {
			end = int64(len(out.Items))
		}
		out.Items = out.Items[start:end]
	}
	return out, nil
}
