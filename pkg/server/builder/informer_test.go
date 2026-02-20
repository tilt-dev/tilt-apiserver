package builder_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientfeatures "k8s.io/client-go/features"
	"k8s.io/client-go/tools/cache"

	corev1alpha1 "github.com/tilt-dev/tilt-apiserver/pkg/apis/core/v1alpha1"
	informers "github.com/tilt-dev/tilt-apiserver/pkg/generated/informers/externalversions"
)

// testFeatureGates overrides specific client-go feature gates while delegating
// all other gate lookups to the original implementation.
type testFeatureGates struct {
	overrides map[clientfeatures.Feature]bool
	defaults  clientfeatures.Gates
}

func (g *testFeatureGates) Enabled(key clientfeatures.Feature) bool {
	if v, ok := g.overrides[key]; ok {
		return v
	}
	return g.defaults.Enabled(key)
}

// overrideFeatureGate replaces the global client-go feature gates for the
// duration of the test, restoring the originals via t.Cleanup.
//
// Note: this modifies global state - tests using this must not run in parallel.
func overrideFeatureGate(t *testing.T, feature clientfeatures.Feature, enabled bool) {
	t.Helper()
	orig := clientfeatures.FeatureGates()
	clientfeatures.ReplaceFeatureGates(&testFeatureGates{
		overrides: map[clientfeatures.Feature]bool{feature: enabled},
		defaults:  orig,
	})
	t.Cleanup(func() {
		clientfeatures.ReplaceFeatureGates(orig)
	})
}

func testInformer(t *testing.T) {

	f := newFixture(t)
	defer f.tearDown()

	_, err := f.client.CoreV1alpha1().Manifests().Create(f.ctx, &corev1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "test-manifest"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	factory := informers.NewSharedInformerFactory(f.client, 0)
	informer := factory.Core().V1alpha1().Manifests().Informer()
	lister := factory.Core().V1alpha1().Manifests().Lister()

	factory.Start(f.ctx.Done())

	syncCtx, syncCancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer syncCancel()

	synced := cache.WaitForCacheSync(syncCtx.Done(), informer.HasSynced)
	require.True(t, synced, "informer cache should sync when WatchListClient is disabled")

	manifests, err := lister.List(labels.Everything())
	require.NoError(t, err)

	names := make([]string, 0, len(manifests))
	for _, m := range manifests {
		names = append(names, m.Name)
	}
	assert.Contains(t, names, "test-manifest")
}

// TestInformerWithWatchListClientDisabled verifies that an informer syncs
// correctly when the WatchListClient feature gate is disabled. The informer
// uses a regular List+Watch, which the tilt-apiserver storage supports.
func TestInformerWithWatchListClientDisabled(t *testing.T) {
	overrideFeatureGate(t, clientfeatures.WatchListClient, false)
	testInformer(t)
}

// TestInformerWithWatchListClientEnabled verifies that an informer syncs
// correctly even when the WatchListClient feature gate is enabled.
func TestInformerWithWatchListClientEnabled(t *testing.T) {
	overrideFeatureGate(t, clientfeatures.WatchListClient, true)
	testInformer(t)
}
