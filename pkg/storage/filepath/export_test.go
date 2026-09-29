package filepath

import (
	"context"

	"k8s.io/apiserver/pkg/registry/rest"
)

// PublicationLockRefsForTest is exported so external filepath_test tests can
// deterministically observe queued mutations without adding production test hooks.
func PublicationLockRefsForTest(storage rest.Storage, ctx context.Context, name string) int {
	filepathStorage := storage.(*filepathREST)
	key := filepathStorage.objectFileName(ctx, name)
	locks := &filepathStorage.watchSet.publicationLocks
	locks.mu.Lock()
	defer locks.mu.Unlock()
	entry := locks.entries[key]
	if entry == nil {
		return 0
	}
	return entry.refs
}
