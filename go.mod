module github.com/tilt-dev/tilt-apiserver

go 1.15

require (
	github.com/akutz/memconn v0.1.0
	github.com/armon/consul-api v0.0.0-20180202201655-eb2c6b5be1b6 // indirect
	github.com/go-logr/logr v0.3.0
	github.com/go-logr/zapr v0.2.0 // indirect
	github.com/go-openapi/spec v0.19.3
	github.com/kr/text v0.2.0 // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/sclevine/agouti v3.0.0+incompatible // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/ugorji/go v1.1.4 // indirect
	github.com/xordataexchange/crypt v0.0.3-0.20170626215501-b2862e3d0a77 // indirect
	golang.org/x/tools v0.0.0-20200812195022-5ae4c3c160a0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/apiserver v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/code-generator v0.20.2
	k8s.io/component-base v0.20.2
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.4.0
	k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd
	sigs.k8s.io/controller-runtime v0.8.3
)

replace (
	// Fix breaking change from gnostic
	// https://github.com/google/gnostic/issues/195
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
)