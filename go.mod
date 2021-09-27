module github.com/tilt-dev/tilt-apiserver

go 1.16

require (
	github.com/akutz/memconn v0.1.0
	github.com/go-logr/logr v0.4.0
	github.com/go-openapi/spec v0.19.5 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	k8s.io/api v0.22.0
	k8s.io/apimachinery v0.22.0
	k8s.io/apiserver v0.22.0
	k8s.io/client-go v0.22.0
	k8s.io/code-generator v0.22.0
	k8s.io/component-base v0.22.0
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kube-openapi v0.0.0-20210421082810-95288971da7e
	sigs.k8s.io/controller-runtime v0.9.6
)
