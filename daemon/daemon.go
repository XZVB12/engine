package daemon

// Daemon holds information about the Malice daemon.
type Daemon struct {
}

// NewDaemon sets up everything for the daemon to be able to service
// requests from the webserver.
func NewDaemon(config *config.Config, registryService registry.Service, pluginStore *plugin.Store) (daemon *Daemon, err error) {


}