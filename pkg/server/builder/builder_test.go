package builder_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1alpha1 "github.com/tilt-dev/tilt-apiserver/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt-apiserver/pkg/generated/clientset/versioned"
	tiltopenapi "github.com/tilt-dev/tilt-apiserver/pkg/generated/openapi"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
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
		WithOpenAPIDefinitions("tilt", "0.1.0", tiltopenapi.GetOpenAPIDefinitions)
	options, err := builder.ToServerOptions()
	require.NoError(t, err)
	options.ServingOptions.BindPort = port

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stoppedCh, err := options.RunTiltServer(ctx.Done())
	require.NoError(t, err)

	client, err := versioned.NewForConfig(&rest.Config{
		Host: fmt.Sprintf("http://localhost:%d", port),
	})
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
