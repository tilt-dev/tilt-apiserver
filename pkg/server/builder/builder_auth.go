package builder

import (
	"github.com/spf13/pflag"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/start"
	"k8s.io/klog"
)

// SetDelegateAuthOptional makes delegated authentication and authorization optional, otherwise
// the apiserver won't failing upon missing delegated auth configurations.
func (a *Server) SetDelegateAuthOptional() *Server {
	start.ServerOptionsFns = append(start.ServerOptionsFns, func(o *ServerOptions) *ServerOptions {
		o.RecommendedOptions.Etcd = nil
		o.RecommendedOptions.Authentication.RemoteKubeConfigFileOptional = true
		o.RecommendedOptions.Authorization.RemoteKubeConfigFileOptional = true
		return o
	})
	return a
}

// DisableAuthorization disables delegated authentication and authorization
func (a *Server) DisableAuthorization() *Server {
	start.ServerOptionsFns = append(start.ServerOptionsFns, func(o *ServerOptions) *ServerOptions {
		o.RecommendedOptions.Authorization = nil
		return o
	})
	return a
}

var enablesLocalStandaloneDebugging bool

// WithLocalDebugExtension adds an optional local-debug mode to the apiserver so that it can be tested
// locally without involving a complete kubernetes cluster. A flag named "--standalone-debug-mode" will
// also be added the binary which forcily requires "--bind-address" to be "127.0.0.1" in order to avoid
// security issues.
func (a *Server) WithLocalDebugExtension() *Server {
	start.ServerOptionsFns = append(start.ServerOptionsFns, func(options *ServerOptions) *ServerOptions {
		secureBindingAddr := options.RecommendedOptions.SecureServing.BindAddress.String()
		if enablesLocalStandaloneDebugging {
			if secureBindingAddr != "127.0.0.1" {
				klog.Fatal(`the binding address must be "127.0.0.1" if --standalone-debug-mode is set`)
			}
			options.RecommendedOptions.Authorization = nil
			options.RecommendedOptions.CoreAPI = nil
			options.RecommendedOptions.Admission = nil
		}
		return options
	})
	start.FlagsFns = append(start.FlagsFns, func(fs *pflag.FlagSet) *pflag.FlagSet {
		fs.BoolVar(&enablesLocalStandaloneDebugging, "standalone-debug-mode", false,
			"Under the local-debug mode the apiserver will allow all access to its resources without "+
				"authorizing the requests, this flag is only intended for debugging in your workstation "+
				"and the apiserver will be crashing if its binding address is not 127.0.0.1.")
		return fs
	})
	return a
}
