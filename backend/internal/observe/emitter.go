// Package observe is a deliberately thin seam for emitting operational signals.
//
// The platform has no metrics backend yet — the choice between DataDog, Prometheus
// and friends is tracked in JQ-166. Rather than block on that decision or hardcode a
// vendor, callers emit through Emitter and get structured stdout today. Swapping in a
// real backend later means adding one implementation here and changing the
// constructor at the entrypoint; no call site has to change.
package observe

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"time"
)

// Emitter records a named numeric signal with optional dimensions.
type Emitter interface {
	// Gauge records the current value of a metric.
	Gauge(name string, value float64, tags map[string]string)
	// Count records an occurrence count for a metric over this run.
	Count(name string, value float64, tags map[string]string)
	// Close flushes anything buffered. Backends that push over the network will
	// need it; the stdout implementation does not.
	Close() error
}

// LogEmitter writes one JSON object per metric, so a log pipeline can parse and
// alert on it without any agent installed. Each line carries `signal: "metric"` to
// make the lines trivially greppable apart from ordinary application logging.
type LogEmitter struct {
	out io.Writer
	now func() time.Time
}

// NewLogEmitter returns an Emitter writing structured lines to stdout.
func NewLogEmitter() *LogEmitter {
	return &LogEmitter{out: os.Stdout, now: time.Now}
}

// NewLogEmitterTo returns an Emitter writing structured lines to w. Intended for tests.
func NewLogEmitterTo(w io.Writer) *LogEmitter {
	return &LogEmitter{out: w, now: time.Now}
}

func (e *LogEmitter) emit(kind, name string, value float64, tags map[string]string) {
	record := map[string]any{
		"signal":    "metric",
		"type":      kind,
		"metric":    name,
		"value":     value,
		"timestamp": e.now().UTC().Format(time.RFC3339),
	}
	if len(tags) > 0 {
		// Sorted so a line is stable and diffable across runs.
		keys := make([]string, 0, len(tags))
		for k := range tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ordered := make(map[string]string, len(tags))
		for _, k := range keys {
			ordered[k] = tags[k]
		}
		record["tags"] = ordered
	}

	encoded, err := json.Marshal(record)
	if err != nil {
		// A metric must never take down the job that emitted it.
		fmt.Fprintf(e.out, `{"signal":"metric","type":%q,"metric":%q,"error":"encode failed"}`+"\n", kind, name)
		return
	}
	fmt.Fprintln(e.out, string(encoded))
}

// Gauge records the current value of a metric.
func (e *LogEmitter) Gauge(name string, value float64, tags map[string]string) {
	e.emit("gauge", name, value, tags)
}

// Count records an occurrence count for a metric.
func (e *LogEmitter) Count(name string, value float64, tags map[string]string) {
	e.emit("count", name, value, tags)
}

// Close satisfies Emitter. Nothing is buffered, so it always succeeds.
func (e *LogEmitter) Close() error { return nil }

// NopEmitter discards every signal. Useful in tests and local runs.
type NopEmitter struct{}

func (NopEmitter) Gauge(string, float64, map[string]string) {}
func (NopEmitter) Count(string, float64, map[string]string) {}
func (NopEmitter) Close() error                             { return nil }
