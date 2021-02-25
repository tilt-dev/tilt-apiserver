package builder_test

import (
	"context"
	"testing"

	"github.com/akutz/memconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1alpha1 "github.com/tilt-dev/tilt-apiserver/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt-apiserver/pkg/generated/clientset/versioned"
	tiltopenapi "github.com/tilt-dev/tilt-apiserver/pkg/generated/openapi"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	connProvider := &memconn.Provider{}
	builder := builder.NewServerBuilder().
		WithResourceMemoryStorage(&corev1alpha1.Manifest{}, "data").
		WithOpenAPIDefinitions("tilt", "0.1.0", tiltopenapi.GetOpenAPIDefinitions).
		WithConnProvider(connProvider)
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

func TestUpdateStatusDoesNotUpdateSpec(t *testing.T) {
	connProvider := &memconn.Provider{}
	builder := builder.NewServerBuilder().
		WithResourceMemoryStorage(&corev1alpha1.Manifest{}, "data").
		WithOpenAPIDefinitions("tilt", "0.1.0", tiltopenapi.GetOpenAPIDefinitions).
		WithConnProvider(connProvider)
	options, err := builder.ToServerOptions()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stoppedCh, err := options.RunTiltServer(ctx.Done())
	require.NoError(t, err)

	client, err := versioned.NewForConfig(options.LoopbackClientConfig())
	require.NoError(t, err)

	newObj, err := client.CoreV1alpha1().Manifests().Create(ctx, &corev1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	newObj.Spec.Message = "spec message"
	newObj.Status.Message = "status message"
	_, err = client.CoreV1alpha1().Manifests().UpdateStatus(ctx, newObj, metav1.UpdateOptions{})
	require.NoError(t, err)

	obj, err := client.CoreV1alpha1().Manifests().Get(ctx, "my-server", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, obj.Name, "my-server")
	assert.False(t, obj.CreationTimestamp.Time.IsZero())
	assert.Equal(t, "", obj.Spec.Message)
	assert.Equal(t, "status message", obj.Status.Message)

	cancel()
	<-stoppedCh
}

func TestUpdateSpectDoesNotUpdateStatus(t *testing.T) {
	connProvider := &memconn.Provider{}
	builder := builder.NewServerBuilder().
		WithResourceMemoryStorage(&corev1alpha1.Manifest{}, "data").
		WithOpenAPIDefinitions("tilt", "0.1.0", tiltopenapi.GetOpenAPIDefinitions).
		WithConnProvider(connProvider)
	options, err := builder.ToServerOptions()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stoppedCh, err := options.RunTiltServer(ctx.Done())
	require.NoError(t, err)

	client, err := versioned.NewForConfig(options.LoopbackClientConfig())
	require.NoError(t, err)

	newObj, err := client.CoreV1alpha1().Manifests().Create(ctx, &corev1alpha1.Manifest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server"},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	newObj.Spec.Message = "spec message"
	newObj.Status.Message = "status message"
	_, err = client.CoreV1alpha1().Manifests().Update(ctx, newObj, metav1.UpdateOptions{})
	require.NoError(t, err)

	obj, err := client.CoreV1alpha1().Manifests().Get(ctx, "my-server", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, obj.Name, "my-server")
	assert.False(t, obj.CreationTimestamp.Time.IsZero())
	assert.Equal(t, "spec message", obj.Spec.Message)
	assert.Equal(t, "", obj.Status.Message)

	cancel()
	<-stoppedCh
}
