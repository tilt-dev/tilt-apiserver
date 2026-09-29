package filepath

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/wait"
)

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
