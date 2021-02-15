package filepath_test

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/tilt-apiserver/pkg/apis/core/v1alpha1"
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

func TestReadEmpty(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	_, err := f.storage.Get(context.Background(), "my-manifest", &metav1.GetOptions{})
	if assert.Error(t, err) {
		assert.True(t, apierrors.IsNotFound(err))
	}
}

func TestCreateThenRead(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	_, err := f.storage.Create(context.Background(), &Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-manifest"},
	}, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	obj, err := f.storage.Get(context.Background(), "my-manifest", &metav1.GetOptions{})
	require.NoError(t, err)

	createdAt := obj.(*Manifest).ObjectMeta.CreationTimestamp.Time
	assert.True(t, !createdAt.IsZero())
}

func TestCreateThenList(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	obj, err := f.storage.List(context.Background(), nil)
	require.NoError(t, err)

	manifestList := obj.(*ManifestList)
	assert.Equal(t, 0, len(manifestList.Items))

	_, err = f.storage.Create(context.Background(), &Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-manifest"},
	}, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	obj, err = f.storage.List(context.Background(), nil)
	require.NoError(t, err)

	manifestList = obj.(*ManifestList)
	assert.Equal(t, 1, len(manifestList.Items))
}

func TestCreateThenReadThenDelete(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	_, err := f.storage.Create(context.Background(), &Manifest{
		TypeMeta:   metav1.TypeMeta{Kind: "Manifest", APIVersion: "core.tilt.dev/v1alpha1"},
		ObjectMeta: metav1.ObjectMeta{Name: "my-manifest"},
	}, nil, &metav1.CreateOptions{})
	require.NoError(t, err)

	obj, err := f.storage.Get(context.Background(), "my-manifest", &metav1.GetOptions{})
	require.NoError(t, err)

	createdAt := obj.(*Manifest).ObjectMeta.CreationTimestamp.Time
	assert.True(t, !createdAt.IsZero())

	_, _, err = f.storage.Delete(context.Background(), "my-manifest", nil, &metav1.DeleteOptions{})
	require.NoError(t, err)

	_, err = f.storage.Get(context.Background(), "my-manifest", &metav1.GetOptions{})
	if assert.Error(t, err) {
		assert.True(t, apierrors.IsNotFound(err))
	}
}

type fixture struct {
	dir     string
	storage rest.StandardStorage
}

func newFixture(t *testing.T) *fixture {
	dir, err := ioutil.TempDir("", t.Name())
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

	provider := filepath.NewJSONFilepathStorageProvider(&Manifest{}, dir, filepath.RealFS{})
	storage, err := provider(scheme, options)
	require.NoError(t, err)

	return &fixture{
		dir:     dir,
		storage: storage.(rest.StandardStorage),
	}
}

func (f *fixture) TearDown() {
	_ = os.Remove(f.dir)
}
