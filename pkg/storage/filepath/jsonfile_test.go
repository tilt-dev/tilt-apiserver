package filepath_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/tilt-apiserver/pkg/apis/core/v1alpha1"
	builderrest "github.com/tilt-dev/tilt-apiserver/pkg/server/builder/rest"
	"github.com/tilt-dev/tilt-apiserver/pkg/storage/filepath"
	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
)

type Manifest = v1alpha1.Manifest
type ManifestList = v1alpha1.ManifestList

func fileSystems() []filepath.FS {
	return []filepath.FS{filepath.NewRealFS(), filepath.NewMemoryFS()}
}

func TestReadEmpty(t *testing.T) {
	for _, fs := range fileSystems() {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			f := newFixture(t, fs)
			defer f.TearDown()
			f.TestReadEmpty()
		})
	}
}

func TestCreateThenRead(t *testing.T) {
	for _, fs := range fileSystems() {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			f := newFixture(t, fs)
			defer f.TearDown()
			f.TestCreateThenRead()
		})
	}
}

func TestCreateThenList(t *testing.T) {
	for _, fs := range fileSystems() {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			f := newFixture(t, fs)
			defer f.TearDown()
			f.TestCreateThenList()
		})
	}
}

func TestCreateThenReadThenDelete(t *testing.T) {
	for _, fs := range fileSystems() {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			f := newFixture(t, fs)
			defer f.TearDown()
			f.TestCreateThenReadThenDelete()
		})
	}
}

func TestDelete(t *testing.T) {
	for _, fs := range fileSystems() {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			f := newFixture(t, fs)
			defer f.TearDown()
			f.TestDelete()
		})
	}
}

func TestListLabelSelector(t *testing.T) {
	for _, fs := range fileSystems() {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			f := newFixture(t, fs)
			defer f.TearDown()
			f.TestListLabelSelector()
		})
	}
}

func TestWatchLabelSelector(t *testing.T) {
	for _, fs := range fileSystems() {
		t.Run(fmt.Sprintf("%T", fs), func(t *testing.T) {
			f := newFixture(t, fs)
			defer f.TearDown()
			f.TestWatchLabelSelector()
		})
	}
}

type fixture struct {
	t       *testing.T
	dir     string
	storage rest.StandardStorage
	ctx     context.Context
	cancel  context.CancelFunc
}

func newFixture(t *testing.T, fs filepath.FS) *fixture {
	dir, err := ioutil.TempDir("", strings.Replace(t.Name(), "/", "_", -1))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	ctx = genericapirequest.WithNamespace(ctx, metav1.NamespaceNone)

	scheme := runtime.NewScheme()
	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	codec := serializer.NewCodecFactory(scheme).LegacyCodec(v1alpha1.SchemeGroupVersion)
	storageConfig := storagebackend.Config{
		Codec: codec,
	}
	options := &options.StorageFactoryRestOptionsFactory{
		Options: options.EtcdOptions{
			StorageConfig: storageConfig,
		},
		StorageFactory: &options.SimpleStorageFactory{StorageConfig: storageConfig},
	}

	ws := filepath.NewWatchSet()
	strategy := builderrest.DefaultStrategy{ObjectTyper: scheme, Object: &Manifest{}}
	provider := filepath.NewJSONFilepathStorageProvider(&Manifest{}, dir, fs, ws, strategy)
	storage, err := provider(scheme, options)
	require.NoError(t, err)

	return &fixture{
		t:       t,
		dir:     dir,
		storage: storage.(rest.StandardStorage),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (f *fixture) TestReadEmpty() {
	_, err := f.storage.Get(f.ctx, "my-manifest", &metav1.GetOptions{})
	if assert.Error(f.t, err) {
		assert.True(f.t, apierrors.IsNotFound(err))
	}
}

func (f *fixture) TestCreateThenRead() {
	_, err := f.storage.Create(f.ctx, &Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-manifest"},
	}, nil, &metav1.CreateOptions{})
	require.NoError(f.t, err)

	obj, err := f.storage.Get(f.ctx, "my-manifest", &metav1.GetOptions{})
	require.NoError(f.t, err)

	createdAt := obj.(*Manifest).ObjectMeta.CreationTimestamp.Time
	assert.True(f.t, !createdAt.IsZero())
}

func (f *fixture) TestCreateThenList() {
	obj, err := f.storage.List(f.ctx, nil)
	require.NoError(f.t, err)

	manifestList := obj.(*ManifestList)
	assert.Equal(f.t, 0, len(manifestList.Items))

	_, err = f.storage.Create(f.ctx, &Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-manifest"},
	}, nil, &metav1.CreateOptions{})
	require.NoError(f.t, err)

	obj, err = f.storage.List(f.ctx, nil)
	require.NoError(f.t, err)

	manifestList = obj.(*ManifestList)
	assert.Equal(f.t, 1, len(manifestList.Items))
	assert.Equal(f.t, manifestList.Items[0].ResourceVersion, manifestList.ResourceVersion)
	assert.NotEqual(f.t, "", manifestList.ResourceVersion)
}

func (f *fixture) TestCreateThenReadThenDelete() {
	_, err := f.storage.Create(f.ctx, &Manifest{
		TypeMeta:   metav1.TypeMeta{Kind: "Manifest", APIVersion: "core.tilt.dev/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{Name: "my-manifest"},
	}, nil, &metav1.CreateOptions{})
	require.NoError(f.t, err)

	obj, err := f.storage.Get(f.ctx, "my-manifest", &metav1.GetOptions{})
	require.NoError(f.t, err)

	createdAt := obj.(*Manifest).ObjectMeta.CreationTimestamp.Time
	assert.True(f.t, !createdAt.IsZero())

	_, _, err = f.storage.Delete(f.ctx, "my-manifest", nil, &metav1.DeleteOptions{})
	require.NoError(f.t, err)

	_, err = f.storage.Get(f.ctx, "my-manifest", &metav1.GetOptions{})
	if assert.Error(f.t, err) {
		assert.True(f.t, apierrors.IsNotFound(err))
	}
}

func (f *fixture) TestDelete() {
	_, _, err := f.storage.Delete(f.ctx, "my-manifest", nil, &metav1.DeleteOptions{})
	if assert.Error(f.t, err) {
		assert.True(f.t, apierrors.IsNotFound(err))
	}
}

func (f *fixture) TestListLabelSelector() {
	_, err := f.storage.Create(f.ctx, &Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "foo-1", Labels: map[string]string{"group": "foo"}},
	}, nil, &metav1.CreateOptions{})
	require.NoError(f.t, err)
	_, err = f.storage.Create(f.ctx, &Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "bar-1", Labels: map[string]string{"group": "bar"}},
	}, nil, &metav1.CreateOptions{})
	require.NoError(f.t, err)

	list, err := f.storage.List(f.ctx, &internalversion.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{"group": "bar"}),
	})
	require.NoError(f.t, err)

	mList := list.(*ManifestList)
	require.Equal(f.t, 1, len(mList.Items))
	assert.Equal(f.t, "bar-1", mList.Items[0].Name)
}

func (f *fixture) TestWatchLabelSelector() {
	ctx := f.ctx
	w, err := f.storage.Watch(ctx, &internalversion.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{"group": "foo"}),
	})
	require.NoError(f.t, err)
	defer w.Stop()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		_, err = f.storage.Create(ctx, &Manifest{
			ObjectMeta: metav1.ObjectMeta{Name: "foo-1", Labels: map[string]string{"group": "foo"}},
		}, nil, &metav1.CreateOptions{})

		if err != nil {
			return err
		}

		_, err = f.storage.Create(ctx, &Manifest{
			ObjectMeta: metav1.ObjectMeta{Name: "bar-1", Labels: map[string]string{"group": "bar"}},
		}, nil, &metav1.CreateOptions{})
		if err != nil {
			return err
		}

		_, err = f.storage.Create(ctx, &Manifest{
			ObjectMeta: metav1.ObjectMeta{Name: "foo-2", Labels: map[string]string{"group": "foo"}},
		}, nil, &metav1.CreateOptions{})
		if err != nil {
			return err
		}
		return nil
	})

	evt := <-w.ResultChan()
	assert.Equal(f.t, "foo-1", evt.Object.(*Manifest).Name)
	evt = <-w.ResultChan()
	assert.Equal(f.t, "foo-2", evt.Object.(*Manifest).Name)

	require.NoError(f.t, g.Wait())
}

func (f *fixture) TearDown() {
	f.cancel()
	_ = os.Remove(f.dir)
}
