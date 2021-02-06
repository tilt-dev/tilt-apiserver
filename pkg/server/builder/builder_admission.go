package builder

import "github.com/tilt-dev/tilt-apiserver/pkg/server/start"

// DisableAdmissionControllers disables delegated authentication and authorization
func (a *Server) DisableAdmissionControllers() *Server {
	start.ServerOptionsFns = append(start.ServerOptionsFns, func(o *ServerOptions) *ServerOptions {
		o.RecommendedOptions.Admission = nil
		return o
	})
	return a
}
