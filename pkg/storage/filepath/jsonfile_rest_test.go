package filepath_test

import (
	"context"
	"io/ioutil"
	"strings"
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
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/storagebackend"

	"github.com/tilt-dev/tilt-apiserver/pkg/apis/core/v1alpha1"
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

type restOptionsGetter struct {
	codec runtime.Codec
}

func (r restOptionsGetter) GetRESTOptions(_ schema.GroupResource) (generic.RESTOptions, error) {
	return generic.RESTOptions{
		StorageConfig: &storagebackend.Config{
			Codec: r.codec,
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

	fs := filepath.NewMemoryFS()

	dir, err := ioutil.TempDir("", strings.Replace(t.Name(), "/", "_", -1))
	require.NoError(t, err)

	scheme := runtime.NewScheme()
	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	obj := v1alpha1.Manifest{}

	sp := filepath.NewJSONFilepathStorageProvider(
		&obj,
		dir,
		fs)

	codec := serializer.NewCodecFactory(scheme).LegacyCodec(v1alpha1.SchemeGroupVersion)
	opts := &restOptionsGetter{codec: codec}

	rootCtx, cancel := context.WithCancel(context.Background())

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

func (r *restFixture) mustNotExist(name string) {
	r.t.Helper()
	ctx, cancel := r.ctx()
	defer cancel()
	_, err := r.getter().Get(ctx, name, nil)
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

func (r *restFixture) mustUpdate(name string, updateFn objectUpdateFn) runtime.Object {
	r.t.Helper()
	ctx, cancel := r.ctx()
	defer cancel()

	updater := objectUpdater{updateFn: updateFn}

	updatedObj, created, err := r.updater().Update(ctx, name, updater, nil, nil, false, nil)
	require.NoError(r.t, err)
	require.False(r.t, created)
	objMeta, err := meta.Accessor(updatedObj)
	require.NoError(r.t, err)
	assert.Equal(r.t, "test-obj", objMeta.GetName())
	return updatedObj
}
