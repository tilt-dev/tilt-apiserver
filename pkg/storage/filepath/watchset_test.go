package filepath

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
)

func TestWatchSetAcceptsConfiguredBacklogAndClosesOnOverflow(t *testing.T) {
	for _, capacity := range []int{1, 7, DefaultWatchBufferSize, 256} {
		t.Run(strconv.Itoa(capacity), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := NewWatchSet()
				require.NoError(t, s.SetBufferSize(capacity))
				w := startTestWatch(t, s, newSelectionPredicate(nil), []watch.Event{watchSetEvent(0), watchSetEvent(1)})
				synctest.Wait()

				for i := range capacity {
					s.notifyWatchers(watchSetEvent(i + 2))
				}
				for i := range capacity + 2 {
					require.Equal(t, watchSetEvent(i), nextWatchSetEvent(t, w))
				}
				next := watchSetEvent(capacity + 2)
				s.notifyWatchers(next)
				require.Equal(t, next, nextWatchSetEvent(t, w))
				w.Stop()

				w = startTestWatch(t, s, newSelectionPredicate(nil), []watch.Event{watchSetEvent(0), watchSetEvent(1)})
				synctest.Wait()
				for i := range capacity + 1 {
					s.notifyWatchers(watchSetEvent(i + 2))
				}
				synctest.Wait()
				waitForWatchSetClose(t, w)
			})
		})
	}
}

func TestWatchSetRejectsNonpositiveBufferSize(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		t.Run(strconv.Itoa(capacity), func(t *testing.T) {
			s := NewWatchSet()
			require.ErrorContains(t, s.SetBufferSize(capacity), "watch buffer size must be positive")
			w := startTestWatch(t, s, newSelectionPredicate(nil), nil)
			ev := watchSetEvent(0)
			s.notifyWatchers(ev)
			require.Equal(t, ev, nextWatchSetEvent(t, w))
		})
	}
}

func TestWatchSetRejectsCapacityChangesAfterWatchCreation(t *testing.T) {
	s := NewWatchSet()
	require.NoError(t, s.SetBufferSize(7))
	w := startTestWatch(t, s, newSelectionPredicate(nil), nil)
	require.NoError(t, s.SetBufferSize(7))
	require.ErrorContains(t, s.SetBufferSize(8), "cannot change after a watch has been created")
	w.Stop()
	require.ErrorContains(t, s.SetBufferSize(8), "cannot change after a watch has been created")
}

func TestWatchSetEventsAreIndependentOfProducerAndOtherWatchers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := NewWatchSet()
		first := startTestWatch(t, s, newSelectionPredicate(nil), nil)
		second := startTestWatch(t, s, newSelectionPredicate(nil), nil)
		event := watchSetEvent(1)
		s.notifyWatchers(event)

		producerObject := event.Object.(*metav1.PartialObjectMetadata)
		producerObject.ResourceVersion = "2"
		producerObject.Labels["app"] = "changed"
		firstObject := nextWatchSetEvent(t, first).Object.(*metav1.PartialObjectMetadata)
		require.Equal(t, "1", firstObject.ResourceVersion)
		require.Equal(t, "test", firstObject.Labels["app"])

		firstObject.Kind = "Manifest"
		firstObject.Labels["app"] = "another change"
		secondObject := nextWatchSetEvent(t, second).Object.(*metav1.PartialObjectMetadata)
		require.Empty(t, secondObject.Kind)
		require.Equal(t, "test", secondObject.Labels["app"])
		require.Empty(t, producerObject.Kind)
	})
}

func TestWatchSetSerializesPublicationForMatchingKeys(t *testing.T) {
	watchSet := NewWatchSet()
	locks := &watchSet.publicationLocks

	unlockFirst := watchSet.lockObjectPublication("first")

	firstAcquired := make(chan func(), 1)
	go func() {
		firstAcquired <- watchSet.lockObjectPublication("first")
	}()

	secondAcquired := make(chan func(), 1)
	go func() {
		secondAcquired <- watchSet.lockObjectPublication("second")
	}()

	waitErr := wait.PollUntilContextTimeout(context.Background(), 200*time.Millisecond, 10*time.Second, true, func(context.Context) (bool, error) {
		locks.mu.Lock()
		defer locks.mu.Unlock()
		first := locks.entries["first"]
		second := locks.entries["second"]
		return first != nil && first.refs == 2 && second != nil && second.refs == 1, nil
	})
	require.NoError(t, waitErr, "Publication locks were not acquired")

	select {
	case unlockSecond := <-secondAcquired:
		unlockSecond()
	case <-time.After(10 * time.Second):
		t.Fatal("Different key was blocked")
	}

	select {
	case unlock := <-firstAcquired:
		unlock()
		t.Fatal("Matching key was not blocked")
	default:
	}

	unlockFirst()

	select {
	case unlock := <-firstAcquired:
		unlock()
	case <-time.After(10 * time.Second):
		t.Fatal("Matching key did not acquire after release")
	}

	locks.mu.Lock()
	defer locks.mu.Unlock()
	require.Empty(t, locks.entries)
}

func TestWatchSetDeliversInitialAndLiveEventsInFIFOOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := NewWatchSet()
		initial := []watch.Event{watchSetEvent(0), watchSetEvent(1)}
		w := startTestWatch(t, s, newSelectionPredicate(nil), initial)
		synctest.Wait()

		events := append([]watch.Event{}, initial...)
		for i := range DefaultWatchBufferSize {
			ev := watchSetEvent(i + 2)
			ev.Type = []watch.EventType{watch.Added, watch.Modified, watch.Deleted}[i%3]
			events = append(events, ev)
			s.notifyWatchers(ev)
		}
		for _, expected := range events {
			require.Equal(t, expected, nextWatchSetEvent(t, w))
		}

		ev := watchSetEvent(DefaultWatchBufferSize + 2)
		s.notifyWatchers(ev)
		require.Equal(t, ev, nextWatchSetEvent(t, w))
	})
}

func TestWatchSetClosesOverflowingWatchWithoutInterruptingHealthyWatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := NewWatchSet()
		slow := startTestWatch(t, s, newSelectionPredicate(nil), []watch.Event{watchSetEvent(0), watchSetEvent(1)})
		healthy := startTestWatch(t, s, newSelectionPredicate(nil), nil)
		synctest.Wait()

		for i := range DefaultWatchBufferSize {
			ev := watchSetEvent(i + 2)
			s.notifyWatchers(ev)
			require.Equal(t, ev, nextWatchSetEvent(t, healthy))
		}
		overflow := watchSetEvent(DefaultWatchBufferSize + 2)
		s.notifyWatchers(overflow)
		require.Equal(t, overflow, nextWatchSetEvent(t, healthy))
		synctest.Wait()

		waitForWatchSetClose(t, slow)

		slow.Stop()
		slow.Stop()
		ev := watchSetEvent(DefaultWatchBufferSize + 3)
		s.notifyWatchers(ev)
		require.Equal(t, ev, nextWatchSetEvent(t, healthy))
	})
}

func TestWatchSetStoppingOneWatchDoesNotBlockOrStopUnreadSibling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := NewWatchSet()
		initial := watchSetEvent(0)
		update := watchSetEvent(1)
		slow := startTestWatch(t, s, newSelectionPredicate(nil), []watch.Event{initial})
		sibling := startTestWatch(t, s, newSelectionPredicate(nil), nil)
		synctest.Wait()

		s.notifyWatchers(update)
		require.Equal(t, update, nextWatchSetEvent(t, sibling))
		sibling.Stop()
		synctest.Wait()
		waitForWatchSetClose(t, sibling)
		require.Equal(t, initial, nextWatchSetEvent(t, slow))
		require.Equal(t, update, nextWatchSetEvent(t, slow))

		restarted := startTestWatch(t, s, newSelectionPredicate(nil), nil)
		afterRestart := watchSetEvent(2)
		s.notifyWatchers(afterRestart)
		require.Equal(t, afterRestart, nextWatchSetEvent(t, restarted))
		require.Equal(t, afterRestart, nextWatchSetEvent(t, slow))
	})
}

func TestWatchSetStopAndContextCancellationCloseUnreadWatch(t *testing.T) {
	for _, cancellation := range []string{"stop", "context"} {
		t.Run(cancellation, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s := NewWatchSet()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				w := s.newWatch(ctx)
				require.NoError(t, w.Start(newSelectionPredicate(nil), func() ([]watch.Event, error) {
					return []watch.Event{watchSetEvent(0)}, nil
				}))
				defer w.Stop()
				synctest.Wait()

				for i := range 5 {
					s.notifyWatchers(watchSetEvent(i + 1))
				}
				synctest.Wait()
				if cancellation == "stop" {
					w.Stop()
					require.NoError(t, ctx.Err())
				} else {
					cancel()
				}
				synctest.Wait()
				waitForWatchSetClose(t, w)
			})
		})
	}
}

func TestWatchSetUnmatchedTrafficDoesNotCloseWatchOrAppearInResults(t *testing.T) {
	for _, selector := range []string{"field", "label"} {
		t.Run(selector, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				p := newSelectionPredicate(nil)
				if selector == "field" {
					p.Field = fields.OneTermEqualSelector("metadata.name", "test-obj")
				} else {
					p.Label = labels.SelectorFromSet(labels.Set{"app": "test"})
				}
				s := NewWatchSet()
				w := startTestWatch(t, s, p, []watch.Event{watchSetEvent(0), watchSetEvent(1)})
				synctest.Wait()

				for i := range DefaultWatchBufferSize + 1 {
					ev := watchSetEvent(i + 2)
					obj := ev.Object.(*metav1.PartialObjectMetadata)
					obj.Name = "unrelated"
					obj.Labels = nil
					s.notifyWatchers(ev)
				}
				ev := watchSetEvent(2)
				s.notifyWatchers(ev)
				require.Equal(t, watchSetEvent(0), nextWatchSetEvent(t, w))
				require.Equal(t, watchSetEvent(1), nextWatchSetEvent(t, w))
				require.Equal(t, ev, nextWatchSetEvent(t, w))
			})
		})
	}
}

func TestWatchSetConcurrentPublicationAndRepeatedStopComplete(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := NewWatchSet()
		w := startTestWatch(t, s, newSelectionPredicate(nil), nil)
		var workers sync.WaitGroup
		for i := range 20 {
			workers.Go(func() {
				for j := range DefaultWatchBufferSize + 1 {
					s.notifyWatchers(watchSetEvent(i*DefaultWatchBufferSize + j))
				}
			})
			workers.Go(w.Stop)
		}
		workers.Wait()
		synctest.Wait()
		waitForWatchSetClose(t, w)
	})
}

func TestWatchSetFailedStartupReturnsErrorAndDoesNotRetainWatch(t *testing.T) {
	s := NewWatchSet()
	w := s.newWatch(context.Background())
	expected := errors.New("initial snapshot failed")
	err := w.Start(newSelectionPredicate(nil), func() ([]watch.Event, error) {
		return nil, expected
	})
	require.ErrorIs(t, err, expected)
	// Failed startup must not retain a watch while the resource is idle.
	require.Empty(t, s.nodes)
	waitForWatchSetClose(t, w)
	healthy := startTestWatch(t, s, newSelectionPredicate(nil), nil)
	ev := watchSetEvent(0)
	s.notifyWatchers(ev)
	require.Equal(t, ev, nextWatchSetEvent(t, healthy))
	w.Stop()
	w.Stop()
}

func startTestWatch(t *testing.T, s *WatchSet, p storage.SelectionPredicate, initial []watch.Event) *watchNode {
	t.Helper()
	w := s.newWatch(context.Background())
	require.NoError(t, w.Start(p, func() ([]watch.Event, error) {
		return initial, nil
	}))
	t.Cleanup(w.Stop)
	return w
}

func watchSetEvent(version int) watch.Event {
	return watch.Event{
		Type: watch.Modified,
		Object: &metav1.PartialObjectMetadata{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-obj",
				ResourceVersion: strconv.Itoa(version),
				Labels:          map[string]string{"app": "test"},
			},
		},
	}
}

func nextWatchSetEvent(t *testing.T, w watch.Interface) watch.Event {
	t.Helper()
	select {
	case ev, ok := <-w.ResultChan():
		require.True(t, ok, "Watch result channel closed")
		return ev
	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for watch event")
		return watch.Event{}
	}
}

func waitForWatchSetClose(t *testing.T, w watch.Interface) {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case _, open := <-w.ResultChan():
			if !open {
				return
			}
		case <-timeout:
			t.Fatal("Timed out waiting for watch to close")
		}
	}
}
