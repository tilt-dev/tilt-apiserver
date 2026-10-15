package filepath

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/klog/v2"
)

// defaultWatchBufferSize is the default number of pending live events per watcher.
const defaultWatchBufferSize = 100

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
		bufferSize: defaultWatchBufferSize,
	}
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

// Publishes an event to every watch that selects it,
// detaching any watch that can't accept any more events.
func (s *WatchSet) notifyWatchers(ev watch.Event) {
	var evicted []*watchNode

	s.mu.Lock()
	for id, w := range s.nodes {
		if !w.enqueue(ev) {
			delete(s.nodes, id)
			evicted = append(evicted, w)
		}
	}
	s.mu.Unlock()

	// Stop the evicted watches outside the lock, because Stop() takes it.
	for _, w := range evicted {
		active := w.ctx.Err() == nil
		w.cancel()
		if active {
			klog.InfoS("Terminating filepath watch because its event buffer is full", "watcher", w.id, "capacity", cap(w.updateCh))
		}
	}
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

// Hands an event to this watch's delivery goroutine.
//
// Returns false if the watch is too far behind and needs to be killed.
//
// Must not block: the caller is the goroutine performing the storage mutation,
// and it holds the registry lock.
func (w *watchNode) enqueue(ev watch.Event) bool {
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

	// Each watch needs its own copy.
	copied := *ev.DeepCopy()

	// if the update channel is full, the watch is too far behind.
	select {
	case w.updateCh <- copied:
		return true
	default:
		return false
	}
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
	w.cancel()

	// Drain the events that we're not going to send.
drain:
	for {
		select {
		case <-w.updateCh:
		default:
			break drain
		}
	}

	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	delete(w.s.nodes, w.id)
}

func (w *watchNode) ResultChan() <-chan watch.Event {
	return w.outCh
}
