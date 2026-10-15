package filepath

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/klog/v2"
)

// DefaultWatchBufferSize is the default number of pending live events per watcher.
const DefaultWatchBufferSize = 100

// Keeps track of which watches need to be notified
type WatchSet struct {
	mu               sync.Mutex
	nodes            map[int]*watchNode
	counter          int
	publicationLocks keyedMutex
	bufferSize       int
}

func NewWatchSet() *WatchSet {
	return &WatchSet{
		nodes:      make(map[int]*watchNode, 10),
		bufferSize: DefaultWatchBufferSize,
	}
}

// SetBufferSize configures a positive per-watcher capacity before the first watch
// is created. Once watching begins, only the existing value may be set again.
func (s *WatchSet) SetBufferSize(bufferSize int) error {
	if bufferSize <= 0 {
		return fmt.Errorf("watch buffer size must be positive: %d", bufferSize)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.counter != 0 && s.bufferSize != bufferSize {
		return fmt.Errorf("watch buffer size cannot change after a watch has been created")
	}
	s.bufferSize = bufferSize
	return nil
}

// Creates a new watch with a unique id, but
// does not start sending events to it until start() is called.
func (s *WatchSet) newWatch(ctx context.Context) *watchNode {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++
	ctx, cancel := context.WithCancel(ctx)

	return &watchNode{
		id:       s.counter,
		s:        s,
		ctx:      ctx,
		cancel:   cancel,
		updateCh: make(chan watch.Event, s.bufferSize),
		outCh:    make(chan watch.Event),
	}
}

func (s *WatchSet) notifyWatchers(ev watch.Event) {
	for _, w := range s.enqueueWatchers(ev) {
		active := w.ctx.Err() == nil
		w.cancel()
		if active {
			klog.InfoS("Terminating filepath watch because its event buffer is full", "watcher", w.id, "capacity", cap(w.updateCh))
		}
	}
}

// enqueueWatchers attempts delivery to every watcher and detaches failed ones.
// It returns them for cancellation outside the registry lock, after all enqueue attempts.
func (s *WatchSet) enqueueWatchers(ev watch.Event) []*watchNode {
	s.mu.Lock()
	defer s.mu.Unlock()

	var failed []*watchNode
	for id, w := range s.nodes {
		if !w.enqueue(ev) {
			delete(s.nodes, id)
			failed = append(failed, w)
		}
	}
	return failed
}

func (s *WatchSet) lockObjectPublication(key string) func() {
	return s.publicationLocks.lock(key)
}

type keyedMutex struct {
	mu      sync.Mutex
	entries map[string]*keyedMutexEntry
}

type keyedMutexEntry struct {
	mu   sync.Mutex
	refs int
}

func (m *keyedMutex) lock(key string) func() {
	m.mu.Lock()
	if m.entries == nil {
		m.entries = make(map[string]*keyedMutexEntry)
	}
	entry := m.entries[key]
	if entry == nil {
		entry = &keyedMutexEntry{}
		m.entries[key] = entry
	}
	entry.refs++
	m.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()

		m.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(m.entries, key)
		}
		m.mu.Unlock()
	}
}

type watchNode struct {
	mu       sync.Mutex
	s        *WatchSet
	p        storage.SelectionPredicate
	id       int
	ctx      context.Context
	cancel   context.CancelFunc
	updateCh chan watch.Event
	outCh    chan watch.Event
}

// Start sending events to this watch.
func (w *watchNode) Start(p storage.SelectionPredicate, initEventFactory func() ([]watch.Event, error)) error {
	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	initEvents, err := initEventFactory()
	if err != nil {
		w.cancel()
		close(w.outCh)
		return err
	}
	w.p = p
	w.s.nodes[w.id] = w

	go w.run(initEvents)
	return nil
}

// enqueue reports whether the watcher is still registered for delivery.
func (w *watchNode) enqueue(ev watch.Event) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ctx.Err() != nil {
		return false
	}
	ok, err := w.p.Matches(ev.Object)
	if err != nil {
		klog.ErrorS(err, "Failed to match filepath watch event", "watcher", w.id)
		return true
	}
	if !ok {
		return true
	}

	if len(w.updateCh) == cap(w.updateCh) {
		return false
	}
	// Writers hold mu, and the reader can only free space while we copy.
	// Encoders mutate TypeMeta, so each watcher must own its event object.
	w.updateCh <- *ev.DeepCopy()
	return true
}

func (w *watchNode) run(initEvents []watch.Event) {
	defer close(w.outCh)
	defer w.Stop()

	for _, ev := range initEvents {
		if !w.send(ev) {
			return
		}
	}
	for {
		select {
		case <-w.ctx.Done():
			return
		case ev := <-w.updateCh:
			if !w.send(ev) {
				return
			}
		}
	}
}

func (w *watchNode) send(e watch.Event) bool {
	select {
	case <-w.ctx.Done():
		return false
	default:
	}
	select {
	case <-w.ctx.Done():
		return false
	case w.outCh <- e:
		return true
	}
}

func (w *watchNode) Stop() {
	w.mu.Lock()
	w.cancel()
drain:
	for {
		select {
		case <-w.updateCh:
		default:
			break drain
		}
	}
	w.mu.Unlock()

	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	delete(w.s.nodes, w.id)
}

func (w *watchNode) ResultChan() <-chan watch.Event {
	return w.outCh
}
