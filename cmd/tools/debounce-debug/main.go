package main

import (
	"flag"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"sync"
	"time"
)

var (
	clearCacheMutex     sync.Mutex
	clearCacheEvents    int
	lastClearCacheEvent time.Time
	clearCacheTimerSet  bool
	lastClearCache      time.Time
	DebounceAfter       time.Duration
	DebounceMax         time.Duration
	clearCacheTime      int
)

// flags
var (
	f_DebounceAfter  = flag.Int("debounce-after", 100, "debounce-after")
	f_DebounceMax    = flag.Int("debounce-max", 10, "debounce-max")
	f_clearCacheTime = flag.Int("clear-cache-time", 1, "clear cache time")
)

// metrics
var (
	pushDecision = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "pilot",
		Name:      "push_decision",
		Help:      "push decision",
	}, []string{"level", "result"})
)

func init() {
	prometheus.Register(pushDecision)
}

func main() {
	flag.Parse()

	DebounceAfter = time.Duration(*f_DebounceAfter) * time.Millisecond
	DebounceMax = time.Duration(*f_DebounceMax) * time.Second
	clearCacheTime = *f_clearCacheTime

	debugH := prometheus.InstrumentHandler("pilot", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		debounce()
	}))
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			promhttp.Handler().ServeHTTP(w, r)
			return
		}

		debugH.ServeHTTP(w, r)
	})
	http.ListenAndServe(":8080", h)
}

func debouncePush(startDebounce time.Time) {
	clearCacheMutex.Lock()
	since := time.Since(lastClearCacheEvent)
	clearCacheMutex.Unlock()

	if since > 2*DebounceAfter ||
		time.Since(startDebounce) > DebounceMax {

		clearCacheMutex.Lock()
		clearCacheTimerSet = false
		lastClearCache = time.Now()
		clearCacheMutex.Unlock()
		pushDecision.WithLabelValues("debounce", "yes").Inc()
	} else {
		time.AfterFunc(DebounceAfter, func() {
			pushDecision.WithLabelValues("debounce", "no").Inc()
			debouncePush(startDebounce)
		})
	}
}

func debounce() {
	clearCacheMutex.Lock()
	defer clearCacheMutex.Unlock()

	clearCacheEvents++

	if DebounceAfter > 0 {
		lastClearCacheEvent = time.Now()

		if !clearCacheTimerSet {
			clearCacheTimerSet = true
			startDebounce := lastClearCacheEvent
			time.AfterFunc(DebounceAfter, func() {
				debouncePush(startDebounce)
			})
		} // else: debunce in progress - it'll keep delaying the push

		return
	}

	// Old code, for safety
	// If last config change was > 1 second ago, push.
	if time.Since(lastClearCacheEvent) > 1*time.Second {
		lastClearCacheEvent = time.Now()
		lastClearCache = time.Now()

		pushDecision.WithLabelValues("oldchange", "yes").Inc()
		return
	}

	lastClearCacheEvent = time.Now()

	// If last config change was < 1 second ago, but last push is > clearCacheTime ago -
	// also push

	if time.Since(lastClearCache) > time.Duration(clearCacheTime)*time.Second {
		lastClearCache = time.Now()
		pushDecision.WithLabelValues("outdate", "yes").Inc()
		return
	}

	// Last config change was < 1 second ago, and we're continuing to get changes.
	// Set a timer 1 second in the future, to evaluate again.
	// if a timer was already set, don't bother.
	if !clearCacheTimerSet {
		clearCacheTimerSet = true
		time.AfterFunc(1*time.Second, func() {
			clearCacheMutex.Lock()
			clearCacheTimerSet = false
			clearCacheMutex.Unlock()
			pushDecision.WithLabelValues("reset", "yes").Inc()
		})
	}
}
