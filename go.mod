
module github.com/tilt-dev/tilt-apiserver

go 1.15

require (
	github.com/go-logr/logr v0.2.1 // indirect
	github.com/go-logr/zapr v0.2.0 // indirect
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/klog v1.0.0
	sigs.k8s.io/apiserver-runtime v0.0.0-20201103144618-b52895ea8337
	sigs.k8s.io/controller-runtime v0.6.0
)
