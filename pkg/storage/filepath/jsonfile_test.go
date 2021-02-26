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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/storage/storagebackend"
)

type Manifest = v1alpha1.Manifest
type ManifestList = v1alpha1.ManifestList

func fileSystems() []filepath.FS {
	return []filepath.FS{filepath.RealFS{}, filepath.NewMemoryFS()}
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

type fixture struct {
	t       *testing.T
	dir     string
	storage rest.StandardStorage
}

func newFixture(t *testing.T, fs filepath.FS) *fixture {
	dir, err := ioutil.TempDir("", strings.Replace(t.Name(), "/", "_", -1))
	require.NoError(t, err)

	scheme := runtime.NewScheme()
	err = v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	codec := serializer.NewCodecFactory(scheme).LegacyCodec(v1alpha1.SchemeGroupVersion)
	options := &options.SimpleRestOptionsFactory{
		Options: options.EtcdOptions{
			StorageConfig: storagebackend.Config{
				Codec: codec,
			},
		},
	}

	strategy := builderrest.DefaultStrategy{ObjectTyper: scheme, Object: &Manifest{}}
	provider := filepath.NewJSONFilepathStorageProvider(&Manifest{}, dir, fs, strategy)
	storage, err := provider(scheme, options)
	require.NoError(t, err)

	return &fixture{
		t:       t,
		dir:     dir,
		storage: storage.(rest.StandardStorage),
	}
}

func (f *fixture) TestReadEmpty() {
	_, err := f.storage.Get(context.Background(), "my-manifest", &metav1.GetOptions{})
	if assert.Error(f.t, err) {
		assert.True(f.t, apierrors.IsNotFound(err))
	}
}

func (f *fixture) TestCreateThenRead() {
	_, err := f.storage.Create(context.Background(), &Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-manifest"},
	}, nil, &metav1.CreateOptions{})
	require.NoError(f.t, err)

	obj, err := f.storage.Get(context.Background(), "my-manifest", &metav1.GetOptions{})
	require.NoError(f.t, err)

	createdAt := obj.(*Manifest).ObjectMeta.CreationTimestamp.Time
	assert.True(f.t, !createdAt.IsZero())
}

func (f *fixture) TestCreateThenList() {
	obj, err := f.storage.List(context.Background(), nil)
	require.NoError(f.t, err)

	manifestList := obj.(*ManifestList)
	assert.Equal(f.t, 0, len(manifestList.Items))

	_, err = f.storage.Create(context.Background(), &Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-manifest"},
	}, nil, &metav1.CreateOptions{})
	require.NoError(f.t, err)

	obj, err = f.storage.List(context.Background(), nil)
	require.NoError(f.t, err)

	manifestList = obj.(*ManifestList)
	assert.Equal(f.t, 1, len(manifestList.Items))
}

func (f *fixture) TestCreateThenReadThenDelete() {
	_, err := f.storage.Create(context.Background(), &Manifest{
		TypeMeta:   metav1.TypeMeta{Kind: "Manifest", APIVersion: "core.tilt.dev/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{Name: "my-manifest"},
	}, nil, &metav1.CreateOptions{})
	require.NoError(f.t, err)

	obj, err := f.storage.Get(context.Background(), "my-manifest", &metav1.GetOptions{})
	require.NoError(f.t, err)

	createdAt := obj.(*Manifest).ObjectMeta.CreationTimestamp.Time
	assert.True(f.t, !createdAt.IsZero())

	_, _, err = f.storage.Delete(context.Background(), "my-manifest", nil, &metav1.DeleteOptions{})
	require.NoError(f.t, err)

	_, err = f.storage.Get(context.Background(), "my-manifest", &metav1.GetOptions{})
	if assert.Error(f.t, err) {
		assert.True(f.t, apierrors.IsNotFound(err))
	}
}

func (f *fixture) TearDown() {
	_ = os.Remove(f.dir)
}
