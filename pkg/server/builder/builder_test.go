package builder_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/akutz/memconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1alpha1 "github.com/tilt-dev/tilt-apiserver/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt-apiserver/pkg/generated/clientset/versioned"
	tiltopenapi "github.com/tilt-dev/tilt-apiserver/pkg/generated/openapi"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/apiserver"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func TestBindToPort0(t *testing.T) {
	builder := builder.NewServerBuilder().
		WithResourceMemoryStorage(&corev1alpha1.Manifest{}, "data").
		WithOpenAPIDefinitions("tilt", "0.1.0", tiltopenapi.GetOpenAPIDefinitions)

	err := builder.ExecuteCommand()
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "No serve port set")
	}
}

func TestBindToPort9444(t *testing.T) {
	port := 9444
	builder := builder.NewServerBuilder().
		WithResourceMemoryStorage(&corev1alpha1.Manifest{}, "data").
		WithOpenAPIDefinitions("tilt", "0.1.0", tiltopenapi.GetOpenAPIDefinitions).
		WithBindPort(port)
	options, err := builder.ToServerOptions()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stoppedCh, err := options.RunTiltServer(ctx.Done())
	require.NoError(t, err)

	client, err := versioned.NewForConfig(options.LoopbackClientConfig())
	require.NoError(t, err)

	_, err = client.CoreV1alpha1().Manifests().Create(ctx, &corev1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	obj, err := client.CoreV1alpha1().Manifests().Get(ctx, "my-server", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, obj.Name, "my-server")
	assert.False(t, obj.CreationTimestamp.Time.IsZero())

	cancel()
	<-stoppedCh
}

func TestMemConn(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	client := f.client
	_, err := client.CoreV1alpha1().Manifests().Create(f.ctx, &corev1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	obj, err := client.CoreV1alpha1().Manifests().Get(f.ctx, "my-server", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, obj.Name, "my-server")
	assert.False(t, obj.CreationTimestamp.Time.IsZero())
}

func TestUpdateStatusDoesNotUpdateSpec(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	client := f.client
	newObj, err := client.CoreV1alpha1().Manifests().Create(f.ctx, &corev1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	newObj.Spec.Message = "spec message"
	newObj.Status.Message = "status message"
	_, err = client.CoreV1alpha1().Manifests().UpdateStatus(f.ctx, newObj, metav1.UpdateOptions{})
	require.NoError(t, err)

	obj, err := client.CoreV1alpha1().Manifests().Get(f.ctx, "my-server", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, obj.Name, "my-server")
	assert.False(t, obj.CreationTimestamp.Time.IsZero())
	assert.Equal(t, "", obj.Spec.Message)
	assert.Equal(t, "status message", obj.Status.Message)
}

func TestWatchStatusUpdate(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	client := f.client
	newObj, err := client.CoreV1alpha1().Manifests().Create(f.ctx, &corev1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	watch, err := client.CoreV1alpha1().Manifests().Watch(f.ctx, metav1.ListOptions{})
	require.NoError(t, err)
	defer watch.Stop()

	obj := f.nextResult(watch)
	assert.Equal(t, "my-server", obj.Name)

	ch := make(chan error)
	go func() {
		newObj.Status.Message = "status message"
		_, err = client.CoreV1alpha1().Manifests().UpdateStatus(f.ctx, newObj, metav1.UpdateOptions{})
		ch <- err
	}()

	require.NoError(t, <-ch)

	obj = f.nextResult(watch)
	assert.Equal(t, "my-server", obj.Name)
	assert.Equal(t, "status message", obj.Status.Message)
}

func TestUpdateSpectDoesNotUpdateStatus(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	client := f.client
	newObj, err := client.CoreV1alpha1().Manifests().Create(f.ctx, &corev1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	newObj.Spec.Message = "spec message"
	newObj.Status.Message = "status message"
	_, err = client.CoreV1alpha1().Manifests().Update(f.ctx, newObj, metav1.UpdateOptions{})
	require.NoError(t, err)

	obj, err := client.CoreV1alpha1().Manifests().Get(f.ctx, "my-server", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, obj.Name, "my-server")
	assert.False(t, obj.CreationTimestamp.Time.IsZero())
	assert.Equal(t, "spec message", obj.Spec.Message)
	assert.Equal(t, "", obj.Status.Message)
}

func TestDelete(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	client := f.client
	err := client.CoreV1alpha1().Manifests().Delete(f.ctx, "my-server", metav1.DeleteOptions{})
	if assert.Error(t, err) {
		assert.True(t, apierrors.IsNotFound(err), err.Error())
	}
}

type createTestCase struct {
	name       string
	labelKey   string
	labelValue string
	error      string
}

func TestCreateValidation(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	cases := []createTestCase{
		{name: "ok"},

		// These are weird names, but are valid path segment names, and will work OK when sent over HTTP:
		// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#path-segment-names
		// In the future, it might make sense to ban whitespace to avoid user confusion.
		{name: "a b "},
		{name: "..."},
		{name: "ab\n"},

		{name: "", error: "invalid: metadata.name: Required value: name or generateName is required"},
		{name: ".", error: "invalid: metadata.name: Invalid value: \".\": may not be '.'"},
		{name: "..", error: "invalid: metadata.name: Invalid value: \"..\": may not be '..'"},
		{name: "a/b", error: "invalid: metadata.name: Invalid value: \"a/b\": may not contain '/'"},
		{name: "a", labelKey: "/", labelValue: "", error: "metadata.labels: Invalid value: \"/\": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character"},
	}

	for i, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%d-%s", i, c.name), func(t *testing.T) {
			client := f.client
			meta := metav1.ObjectMeta{Name: c.name}
			if c.labelKey != "" {
				meta.Labels = map[string]string{
					c.labelKey: c.labelValue,
				}
			}
			_, err := client.CoreV1alpha1().Manifests().Create(f.ctx, &corev1alpha1.Manifest{
				ObjectMeta: meta,
			}, metav1.CreateOptions{})
			if c.error == "" {
				assert.NoError(t, err)

				obj, err := client.CoreV1alpha1().Manifests().Get(f.ctx, c.name, metav1.GetOptions{})
				assert.NoError(t, err)
				assert.Equal(t, c.name, obj.ObjectMeta.Name)

			} else if assert.Error(t, err) {
				assert.Contains(t, err.Error(), c.error)
			}
		})
	}
}

func TestValidateOpenAPISpec(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	tr := &http.Transport{
		DialContext: f.connProvider.DialContext,
	}
	client := &http.Client{Transport: tr}
	resp, err := client.Get("http://127.0.0.1:80/openapi/v2")
	require.NoError(t, err)
	defer resp.Body.Close()

	contentBytes, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	content := string(contentBytes)
	assert.Contains(t, content, `"operationId":"watchCoreTiltDevV1alpha1Manifest"`)
	assert.NotContains(t, content, `"operationId":"watchCoreTiltDevV1alpha1ManifestStatus"`)
}

func memConnProvider() apiserver.ConnProvider {
	return apiserver.NetworkConnProvider(&memconn.Provider{}, "memu")
}

type fixture struct {
	t            *testing.T
	ctx          context.Context
	cancel       func()
	stoppedCh    <-chan struct{}
	client       *versioned.Clientset
	connProvider apiserver.ConnProvider
}

func newFixture(t *testing.T) *fixture {
	connProvider := memConnProvider()
	builder := builder.NewServerBuilder().
		WithResourceMemoryStorage(&corev1alpha1.Manifest{}, "data").
		WithOpenAPIDefinitions("tilt", "0.1.0", tiltopenapi.GetOpenAPIDefinitions).
		WithConnProvider(connProvider)
	options, err := builder.ToServerOptions()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	stoppedCh, err := options.RunTiltServer(ctx.Done())
	require.NoError(t, err)

	client, err := versioned.NewForConfig(options.LoopbackClientConfig())
	require.NoError(t, err)

	return &fixture{
		t:            t,
		ctx:          ctx,
		cancel:       cancel,
		stoppedCh:    stoppedCh,
		client:       client,
		connProvider: connProvider,
	}
}

func (f *fixture) nextResult(i watch.Interface) *corev1alpha1.Manifest {
	select {
	case e := <-i.ResultChan():
		obj := e.Object
		m, ok := obj.(*corev1alpha1.Manifest)
		if !ok {
			require.Failf(f.t, "Unexpected object", "Object type: %T", obj)
			return nil
		}
		return m
	case <-time.After(time.Second):
		require.Fail(f.t, "timeout waiting for next watch result")
		return nil
	}

}

func (f *fixture) tearDown() {
	f.cancel()
	<-f.stoppedCh
}
