package builder

import (
	"github.com/spf13/pflag"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/apiserver"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/start"
)

// WithOptionsFns sets functions to customize the ServerOptions used to create the apiserver
func (a *Server) WithOptionsFns(fns ...func(*ServerOptions) *ServerOptions) *Server {
	start.ServerOptionsFns = append(start.ServerOptionsFns, fns...)
	return a
}

// WithServerFns sets functions to customize the GenericAPIServer
func (a *Server) WithServerFns(fns ...func(server *GenericAPIServer) *GenericAPIServer) *Server {
	apiserver.GenericAPIServerFns = append(apiserver.GenericAPIServerFns, fns...)
	return a
}

// WithFlagFns sets functions to customize the flags for the compiled binary.
func (a *Server) WithFlagFns(fns ...func(set *pflag.FlagSet) *pflag.FlagSet) *Server {
	start.FlagsFns = append(start.FlagsFns, fns...)
	return a
}
