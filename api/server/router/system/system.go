package system

import (
	"github.com/maliceio/engine/api/server/router"
)

// systemRouter provides information about the Docker system overall.
// It gathers information about host, daemon and container events.
type systemRouter struct {
	backend Backend
	// cluster *cluster.Cluster
	routes []router.Route
	// builder *fscache.FSCache
}

// NewRouter initializes a new system router
func NewRouter(b Backend) router.Router {
	r := &systemRouter{
		backend: b,
		// cluster: c,
		// builder: fscache,
	}

	r.routes = []router.Route{
		router.NewOptionsRoute("/{anyroute:.*}", optionsHandler),
		router.NewGetRoute("/_ping", pingHandler),
		router.NewGetRoute("/events", r.getEvents, router.WithCancel),
		router.NewGetRoute("/info", r.getInfo),
		router.NewGetRoute("/version", r.getVersion),
		router.NewGetRoute("/system/df", r.getDiskUsage, router.WithCancel),
		router.NewPostRoute("/auth", r.postAuth),
	}

	return r
}

// Routes returns all the API routes dedicated to the docker system
func (s *systemRouter) Routes() []router.Route {
	return s.routes
}
