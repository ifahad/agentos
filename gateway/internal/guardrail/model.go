package guardrail

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// DefaultModel is the classifier model used when AGENTOS_GUARDRAILS_MODEL is
// unset.
const DefaultModel = "anthropic/claude-haiku-4-5"

const (
	// cacheSize bounds the verdict LRU cache.
	cacheSize = 256
	// classifyTimeout is the default bound for one screening (both classifier
	// attempts). Override with AGENTOS_GUARDRAILS_TIMEOUT_S — slow local
	// reasoning models used as classifiers may need more than the default.
	classifyTimeout = 20 * time.Second
)

// Classifier is the minimal model-backend dependency of ModelScreen. It
// judges whether message attempts prompt injection using the given
// provider-prefixed model. Tests inject a fake; production wires
// ProviderClassifier.
type Classifier interface {
	Classify(ctx context.Context, model, message string) (injection bool, reason string, err error)
}

// ModelScreen is the AGENTOS_GUARDRAILS_MODE=model screener: the heuristic
// runs first (obvious injections block without spending a model call), then
// a classifier model judges the rest. Verdicts are memoized in a bounded LRU
// keyed by the message hash; classifier failures fail open via
// Verdict.Errored.
type ModelScreen struct {
	heuristic  Guardrail
	classifier Classifier
	model      string
	timeout    time.Duration
	cache      *lruCache
}

// NewModelScreen builds the model-backed screener. An empty model falls back
// to DefaultModel.
func NewModelScreen(heuristic Guardrail, classifier Classifier, model string) *ModelScreen {
	return NewModelScreenWithTimeout(heuristic, classifier, model, classifyTimeout)
}

// NewModelScreenWithTimeout is NewModelScreen with an explicit screening
// timeout (bounds both classifier attempts). A non-positive timeout falls
// back to the default.
func NewModelScreenWithTimeout(heuristic Guardrail, classifier Classifier, model string, timeout time.Duration) *ModelScreen {
	if model == "" {
		model = DefaultModel
	}
	if timeout <= 0 {
		timeout = classifyTimeout
	}
	return &ModelScreen{
		heuristic:  heuristic,
		classifier: classifier,
		model:      model,
		timeout:    timeout,
		cache:      newLRUCache(cacheSize),
	}
}

// Screen applies the heuristic short-circuit, then the cached model verdict,
// then the classifier (one retry). Classifier failure returns an Errored
// verdict (fail open) and is never cached.
func (m *ModelScreen) Screen(latestUserMessage string) Verdict {
	if v := m.heuristic.Screen(latestUserMessage); v.Flagged {
		return v // obvious injection: no model call needed
	}
	if strings.TrimSpace(latestUserMessage) == "" {
		return Verdict{}
	}

	key := messageHash(latestUserMessage)
	if v, ok := m.cache.get(key); ok {
		return v
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	injection, reason, err := m.classifier.Classify(ctx, m.model, latestUserMessage)
	if err != nil {
		injection, reason, err = m.classifier.Classify(ctx, m.model, latestUserMessage)
	}
	if err != nil {
		return Verdict{Errored: true, Reason: "classifier: " + err.Error()}
	}

	v := Verdict{}
	if injection {
		if reason == "" {
			reason = "model-classifier"
		}
		v = Verdict{Flagged: true, Reason: reason}
	}
	m.cache.put(key, v)
	return v
}

// messageHash keys the verdict cache: hex SHA-256 of the message text.
func messageHash(message string) string {
	sum := sha256.Sum256([]byte(message))
	return hex.EncodeToString(sum[:])
}

// lruCache is a thread-safe fixed-capacity LRU of screening verdicts.
type lruCache struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List // front = most recently used
	items    map[string]*list.Element
}

// lruEntry is the list payload: the key is kept for O(1) eviction.
type lruEntry struct {
	key     string
	verdict Verdict
}

func newLRUCache(capacity int) *lruCache {
	return &lruCache{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[string]*list.Element, capacity),
	}
}

// get returns the cached verdict and bumps its recency.
func (c *lruCache) get(key string) (Verdict, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return Verdict{}, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*lruEntry).verdict, true
}

// put inserts or refreshes a verdict, evicting the least recently used entry
// past capacity.
func (c *lruCache) put(key string, v Verdict) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		el.Value.(*lruEntry).verdict = v
		c.ll.MoveToFront(el)
		return
	}
	c.items[key] = c.ll.PushFront(&lruEntry{key: key, verdict: v})
	if c.ll.Len() > c.capacity {
		oldest := c.ll.Back()
		c.ll.Remove(oldest)
		delete(c.items, oldest.Value.(*lruEntry).key)
	}
}

// len reports the current entry count (tests only).
func (c *lruCache) len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
