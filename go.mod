module github.com/tilt-dev/tilt-apiserver

go 1.16

require (
	github.com/akutz/memconn v0.1.0
	github.com/go-logr/logr v0.4.0
	github.com/go-openapi/spec v0.19.5
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/apiserver v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/code-generator v0.21.2
	k8s.io/component-base v0.21.2
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.8.0
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	sigs.k8s.io/controller-runtime v0.9.2
)

// Fix breaking change from gnostic
// https://github.com/google/gnostic/issues/195
replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
