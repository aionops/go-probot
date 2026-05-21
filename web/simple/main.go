package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aionops/go-probot/web"
	"github.com/aionops/go-probot/web/backend"
)

func main() {
	web.RegisterHandler(http.DefaultServeMux, &fakeEventSource{})
	fmt.Println("http://127.0.0.1:7771")
	http.ListenAndServe("127.0.0.1:7771", nil)
}

type fakeEventSource struct{}

var _ backend.EventSource = &fakeEventSource{}

func (es *fakeEventSource) ListEvent(ctx context.Context, opts backend.ListOptions) (backend.EventList, error) {
	return backend.EventList{
		Items: []backend.Event{
			{Name: "foo", Type: backend.EventType("foo"), Metrics: backend.EventMetrics{
				Count: 1,
			}},
			{Name: "bar", Type: backend.EventType("bar"), Metrics: backend.EventMetrics{
				Count: 2,
			}},
		},
	}, nil
}
