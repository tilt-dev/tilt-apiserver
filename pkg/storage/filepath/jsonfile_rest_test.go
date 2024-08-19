package filepath_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/storagebackend"

	"github.com/tilt-dev/tilt-apiserver/pkg/apis/core/v1alpha1"
	builderrest "github.com/tilt-dev/tilt-apiserver/pkg/server/builder/rest"
	"github.com/tilt-dev/tilt-apiserver/pkg/storage/filepath"
)

func TestFilepathREST_Delete_NoFinalizers(t *testing.T) {
	f := newRESTFixture(t)
	defer f.tearDown()

	obj := &v1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-obj",
		},
	}

	f.mustCreate(obj)

	w := f.watch("test-obj")
	defer w.Stop()
	// watch always immediately emits ADDED events for pre-existing objects,
	// so just ignore first event
	<-w.ResultChan()

	ctx, cancel := f.ctx()
	defer cancel()

	deletedObj, deletedImmediately, err := f.deleter().Delete(ctx, "test-obj", nil, nil)
	require.NoError(t, err)
	objMeta := f.mustMeta(deletedObj)
	assert.Equal(t, "test-obj", objMeta.GetName())
	assert.Zero(t, objMeta.GetDeletionTimestamp())
	assert.Nil(t, objMeta.GetDeletionGracePeriodSeconds())
	assert.True(t, deletedImmediately)

	e := <-w.ResultChan()
	assert.Equal(t, watch.Deleted, e.Type)

	f.mustNotExist("test-obj")
}

func TestFilepathREST_Delete_Finalizers(t *testing.T) {
	f := newRESTFixture(t)
	defer f.tearDown()

	obj := &v1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-obj",
			Finalizers: []string{"test.tilt.dev"},
		},
	}

	f.mustCreate(obj)

	w := f.watch("test-obj")
	defer w.Stop()
	// watch always immediately emits ADDED events for pre-existing objects,
	// so just ignore first event
	<-w.ResultChan()

	ctx, cancel := f.ctx()
	defer cancel()

	deletedObj, deletedImmediately, err := f.deleter().Delete(ctx, "test-obj", nil, nil)
	require.NoError(t, err)
	objMeta := f.mustMeta(deletedObj)
	assert.Equal(t, "test-obj", objMeta.GetName())
	assert.NotZero(t, objMeta.GetDeletionTimestamp())
	assert.Equal(t, int64(0), *objMeta.GetDeletionGracePeriodSeconds())
	assert.False(t, deletedImmediately)

	e := <-w.ResultChan()
	// because object was soft-deleted, a modified event is actually fired
	// for the deletion timestamp + grace period secs changes
	assert.Equal(t, watch.Modified, e.Type)

	// in a normal scenario, a controller would see the deletion timestamp set, run its finalizer(s),
	// and remove its finalizer(s), triggering another update(s) at which point the entity is finally
	// deleted once no more finalizers remain
	f.mustUpdate("test-obj", func(obj runtime.Object) {
		f.mustMeta(obj).SetFinalizers(nil)
	})

	// at this point, a delete event should be received
	e = <-w.ResultChan()
	assert.Equal(t, watch.Deleted, e.Type)

	f.mustNotExist("test-obj")
}

func TestFilepathREST_Update_OptimisticConcurrency(t *testing.T) {
	f := newRESTFixture(t)
	defer f.tearDown()

	var obj runtime.Object
	obj = &v1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-obj",
		},
		Spec: v1alpha1.ManifestSpec{
			Message: "original",
		},
	}

	f.mustCreate(obj)

	obj = f.mustUpdate("test-obj", func(obj runtime.Object) {
		m := obj.(*v1alpha1.Manifest)
		m.Spec.Message = "updated"
	})

	require.Equal(t, "3", f.mustMeta(obj).GetResourceVersion())
	require.Equal(t, "updated", obj.(*v1alpha1.Manifest).Spec.Message)

	obj, err := f.update("test-obj", func(obj runtime.Object) {
		m := obj.(*v1alpha1.Manifest)
		m.SetResourceVersion("1")
		m.Spec.Message = "impossible"
	})

	require.EqualError(t, err,
		`Operation cannot be fulfilled on manifests.core.tilt.dev "test-obj": the object has been modified; please apply your changes to the latest version and try again`)
	require.Nil(t, obj)

	obj, err = f.get("test-obj")
	require.NoError(t, err, "Failed to fetch object")
	// object should not have changed
	require.Equal(t, "3", f.mustMeta(obj).GetResourceVersion())
	require.Equal(t, "updated", obj.(*v1alpha1.Manifest).Spec.Message)
}

func TestFilepathREST_Update_OptimisticConcurrency_Subresource(t *testing.T) {
	f := newRESTFixtureWithStrategy(t, func(defaultStrategy builderrest.Strategy) builderrest.Strategy {
		return builderrest.StatusSubResourceStrategy{Strategy: defaultStrategy}
	})
	defer f.tearDown()

	var obj runtime.Object
	obj = &v1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-obj",
		},
		Spec: v1alpha1.ManifestSpec{
			Message: "spec_message",
		},
		Status: v1alpha1.ManifestStatus{
			Message: "status_message",
		},
	}

	f.mustCreate(obj)

	obj = f.mustUpdate("test-obj", func(obj runtime.Object) {
		m := obj.(*v1alpha1.Manifest)
		m.Spec.Message = "this_should_be_ignored"
		m.Status.Message = "updated_status_message"
	})

	assert.Equal(t, "3", f.mustMeta(obj).GetResourceVersion())
	assert.Equal(t, "spec_message", obj.(*v1alpha1.Manifest).Spec.Message)
	require.Equal(t, "updated_status_message", obj.(*v1alpha1.Manifest).Status.Message)

	obj, err := f.update("test-obj", func(obj runtime.Object) {
		m := obj.(*v1alpha1.Manifest)
		m.SetResourceVersion("2")
		m.Status.Message = "impossible"
	})

	if assert.EqualError(t, err,
		`Operation cannot be fulfilled on manifests.core.tilt.dev "test-obj": the object has been modified; please apply your changes to the latest version and try again`) {
		assert.Nil(t, obj)
	}

	obj, err = f.get("test-obj")
	require.NoError(t, err, "Failed to fetch object")
	// object should not have changed
	assert.Equal(t, "3", f.mustMeta(obj).GetResourceVersion())
	assert.Equal(t, "updated_status_message", obj.(*v1alpha1.Manifest).Status.Message)
}

func TestFilepathREST_Update_SimultaneousUpdates(t *testing.T) {
	f := newRESTFixture(t)
	defer f.tearDown()

	var obj runtime.Object
	obj = &v1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-obj",
		},
		Spec: v1alpha1.ManifestSpec{
			Message: "original",
		},
	}

	f.mustCreate(obj)

	type result struct {
		inVersion  string
		outVersion string
		message    string
	}

	// create a bunch of workers that loop attempting to do updates and keep
	// track of which are successful so that we can ensure that only one update
	// per input resourceVersion is ever accepted by the server
	const workerCount = 20
	const workerIterations = 100
	var results [workerCount][workerIterations]result
	var wg sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func(worker int) {
			for i := 0; i < workerIterations; i++ {
				var inVersion string
				msg := fmt.Sprintf("worker-%d-iteration-%d", worker, i)
				obj, err := f.update("test-obj", func(obj runtime.Object) {
					m := obj.(*v1alpha1.Manifest)
					m.Spec.Message = msg
					inVersion = m.GetResourceVersion()
				})
				if err == nil {
					m := obj.(*v1alpha1.Manifest)
					// verify the version returned back to us has our data
					require.Equal(t, msg, m.Spec.Message, "Incorrect updated object message")
					results[worker][i] = result{
						inVersion:  inVersion,
						outVersion: m.GetResourceVersion(),
						message:    m.Spec.Message,
					}
				}
			}
			wg.Done()
		}(worker)
	}

	wg.Wait()

	seen := make(map[string]string)
	for worker := range results {
		for i := range results[worker] {
			r := results[worker][i]
			if r.inVersion == "" {
				continue
			}
			if v, ok := seen[r.inVersion]; ok {
				// apiserver accepted > 1 update for the same inVersion
				// NOTE: if this is failing and you see 2x identical outVersions, that's not a test issue! it means
				// 	not only was the update accepted twice, but there are now two _different_ objects out there with
				// 	the same resource version
				t.Fatalf("Saw more than one update for inVersion=%s (outVersion=%s and outVersion=%s)",
					r.inVersion, v, r.outVersion)
			}

			// it IS possible for a no-op update to result in no version change, but all the updates in this test
			// mutate the object, so if the version doesn't change but apiserver accepts the update, that's a bug
			require.NotEqualf(t, r.inVersion, r.outVersion,
				"inVersion and outVersion are equal (apiserver changed object without changing version)")

			seen[r.inVersion] = r.outVersion
			require.Equal(t, fmt.Sprintf("worker-%d-iteration-%d", worker, i), r.message)
		}
	}
}

// https://github.com/tilt-dev/tilt/issues/5541
func TestFilepathREST_UpdateIdentical(t *testing.T) {
	f := newRESTFixture(t)
	defer f.tearDown()

	var obj runtime.Object
	obj = &v1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-obj",
		},
		Spec: v1alpha1.ManifestSpec{
			Message: "original",
		},
	}

	f.mustCreate(obj)

	result, err := f.update("test-obj", func(obj runtime.Object) {})
	require.NoError(t, err)

	// ideally we'd just compare the object after `update` to the object after `create`, but:
	// 1) the result of create doesn't have a populated TypeMeta, and the result of update does
	// 2) the result of update has a truncated CreationTimestamp
	actual := result.(*v1alpha1.Manifest)
	require.Equal(t, "2", actual.ResourceVersion)
}

type restOptionsGetter struct {
	codec runtime.Codec
}

func (r restOptionsGetter) GetRESTOptions(resource schema.GroupResource, obj runtime.Object) (generic.RESTOptions, error) {
	return generic.RESTOptions{
		StorageConfig: &storagebackend.ConfigForResource{
			GroupResource: resource,
			Config: storagebackend.Config{
				Codec: r.codec,
			},
		},
	}, nil
}

type restFixture struct {
	t       *testing.T
	rest    rest.Storage
	rootCtx context.Context
	cancel  context.CancelFunc
}

func newRESTFixture(t *testing.T) *restFixture {
	t.Helper()
	return newRESTFixtureWithStrategy(t, func(defaultStrategy builderrest.Strategy) builderrest.Strategy {
		return defaultStrategy
	})
}

func newRESTFixtureWithStrategy(t *testing.T,
	strategyFn func(defaultStrategy builderrest.Strategy) builderrest.Strategy) *restFixture {
	t.Helper()

	fs := filepath.NewMemoryFS()
	ws := filepath.NewWatchSet()

	dir, err := ioutil.TempDir("", strings.Replace(t.Name(), "/", "_", -1))
	require.NoError(t, err)

	scheme := runtime.NewScheme()
	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	obj := v1alpha1.Manifest{}
	defaultStrategy := builderrest.DefaultStrategy{ObjectTyper: scheme, Object: &obj}

	sp := filepath.NewJSONFilepathStorageProvider(
		&obj,
		dir,
		fs,
		ws,
		strategyFn(defaultStrategy))

	codec := serializer.NewCodecFactory(scheme).LegacyCodec(v1alpha1.SchemeGroupVersion)
	opts := &restOptionsGetter{codec: codec}

	rootCtx, cancel := context.WithCancel(context.Background())
	rootCtx = genericapirequest.WithNamespace(rootCtx, metav1.NamespaceNone)

	storage, err := sp(scheme, opts)
	require.NoError(t, err, "Failed to create storage provider for test setup")
	return &restFixture{
		t:       t,
		rootCtx: rootCtx,
		cancel:  cancel,
		rest:    storage,
	}
}

func (r *restFixture) tearDown() {
	r.t.Helper()
	r.cancel()
}

func (r *restFixture) ctx() (context.Context, context.CancelFunc) {
	r.t.Helper()
	return context.WithTimeout(r.rootCtx, 10*time.Second)
}

func (r *restFixture) creater() rest.Creater {
	r.t.Helper()
	creater, ok := r.rest.(rest.Creater)
	require.True(r.t, ok, "REST storage is not a rest.Creater")
	return creater
}

func (r *restFixture) getter() rest.Getter {
	r.t.Helper()
	getter, ok := r.rest.(rest.Getter)
	require.True(r.t, ok, "REST storage is not a rest.Getter")
	return getter
}

func (r *restFixture) updater() rest.Updater {
	r.t.Helper()
	updater, ok := r.rest.(rest.Updater)
	require.True(r.t, ok, "REST storage is not a rest.Updater")
	return updater
}

func (r *restFixture) deleter() rest.GracefulDeleter {
	r.t.Helper()
	deleter, ok := r.rest.(rest.GracefulDeleter)
	require.True(r.t, ok, "REST storage is not a rest.GracefulDeleter")
	return deleter
}

func (r *restFixture) watcher() rest.Watcher {
	r.t.Helper()
	watcher, ok := r.rest.(rest.Watcher)
	require.True(r.t, ok, "REST storage is not a rest.Watcher")
	return watcher
}

func (r *restFixture) watch(name string) watch.Interface {
	r.t.Helper()
	// N.B. rootCtx is used here so that the watch isn't canceled until teardown
	w, err := r.watcher().Watch(r.rootCtx, &metainternalversion.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
	})
	require.NoError(r.t, err)
	return w
}

func (r *restFixture) mustMeta(obj interface{}) metav1.Object {
	r.t.Helper()
	metaObj, err := meta.Accessor(obj)
	require.NoError(r.t, err)
	return metaObj
}

func (r *restFixture) mustCreate(obj runtime.Object) runtime.Object {
	r.t.Helper()
	ctx, cancel := r.ctx()
	defer cancel()
	createdObj, err := r.creater().Create(ctx, obj, nil, nil)
	require.NoError(r.t, err)
	objMeta, err := meta.Accessor(createdObj)
	require.NoError(r.t, err)
	assert.Equal(r.t, "test-obj", objMeta.GetName())
	return createdObj
}

func (r *restFixture) get(name string) (runtime.Object, error) {
	ctx, cancel := r.ctx()
	defer cancel()
	obj, err := r.getter().Get(ctx, name, nil)
	return obj, err
}

func (r *restFixture) mustNotExist(name string) {
	r.t.Helper()
	_, err := r.get(name)
	apiError, ok := err.(apierrors.APIStatus)
	require.Truef(r.t, ok && apiError.Status().Code == 404, "Did not receive APIStatus not found error: %v", err)
}

type objectUpdateFn func(obj runtime.Object)

type objectUpdater struct {
	updateFn objectUpdateFn
}

func (o objectUpdater) Preconditions() *metav1.Preconditions {
	return nil
}

func (o objectUpdater) UpdatedObject(ctx context.Context, oldObj runtime.Object) (newObj runtime.Object, err error) {
	toUpdate := oldObj.DeepCopyObject()
	o.updateFn(toUpdate)
	return toUpdate, nil
}

func (r *restFixture) update(name string, updateFn objectUpdateFn) (runtime.Object, error) {
	r.t.Helper()
	ctx, cancel := r.ctx()
	defer cancel()

	updater := objectUpdater{updateFn: updateFn}

	updatedObj, created, err := r.updater().Update(ctx, name, updater, nil, nil, false, nil)
	require.False(r.t, created)

	if err == nil {
		objMeta := r.mustMeta(updatedObj)
		assert.Equal(r.t, name, objMeta.GetName())
	}

	return updatedObj, err
}

func (r *restFixture) mustUpdate(name string, updateFn objectUpdateFn) runtime.Object {
	r.t.Helper()
	updatedObj, err := r.update(name, updateFn)
	require.NoErrorf(r.t, err, "Failed to update %s", name)
	return updatedObj
}
