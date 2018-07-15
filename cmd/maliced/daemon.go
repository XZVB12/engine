package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/dockerversion"
	apiserver "github.com/maliceio/engine/api/server"
	"github.com/maliceio/engine/api/server/router"
	"github.com/maliceio/engine/daemon"
	"github.com/maliceio/engine/daemon/config"
	"github.com/maliceio/engine/daemon/listeners"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

// DaemonCli represents the daemon CLI.
type DaemonCli struct {
	*config.Config
	configFile *string
	flags      *pflag.FlagSet

	api *apiserver.Server
	d   *daemon.Daemon
}

// NewDaemonCli returns a daemon CLI
func NewDaemonCli() *DaemonCli {
	return &DaemonCli{}
}

func (cli *DaemonCli) start(opts *daemonOptions) (err error) {
	stopc := make(chan bool)
	defer close(stopc)
	// TODO fill this out
}

type routerOptions struct {
	sessionManager *session.Manager
	daemon         *daemon.Daemon
	api            *apiserver.Server
	// cluster        *cluster.Cluster
}

func newRouterOptions(config *config.Config, daemon *daemon.Daemon) (routerOptions, error) {
	opts := routerOptions{}
	sm, err := session.NewManager()
	if err != nil {
		return opts, errors.Wrap(err, "failed to create sessionmanager")
	}

	return routerOptions{
		sessionManager: sm,
		daemon:         daemon,
	}, nil
}

func (cli *DaemonCli) reloadConfig() {
	reload := func(c *config.Config) {

		// // Revalidate and reload the authorization plugins
		// if err := validateAuthzPlugins(c.AuthorizationPlugins, cli.d.PluginStore); err != nil {
		// 	logrus.Fatalf("Error validating authorization plugin: %v", err)
		// 	return
		// }
		// cli.authzMiddleware.SetPlugins(c.AuthorizationPlugins)

		// // The namespaces com.docker.*, io.docker.*, org.dockerproject.* have been documented
		// // to be reserved for Docker's internal use, but this was never enforced.  Allowing
		// // configured labels to use these namespaces are deprecated for 18.05.
		// //
		// // The following will check the usage of such labels, and report a warning for deprecation.
		// //
		// // TODO: At the next stable release, the validation should be folded into the other
		// // configuration validation functions and an error will be returned instead, and this
		// // block should be deleted.
		// if err := config.ValidateReservedNamespaceLabels(c.Labels); err != nil {
		// 	logrus.Warnf("Configured labels using reserved namespaces is deprecated: %s", err)
		// }

		// if err := cli.d.Reload(c); err != nil {
		// 	logrus.Errorf("Error reconfiguring the daemon: %v", err)
		// 	return
		// }

		// if c.IsValueSet("debug") {
		// 	debugEnabled := debug.IsEnabled()
		// 	switch {
		// 	case debugEnabled && !c.Debug: // disable debug
		// 		debug.Disable()
		// 	case c.Debug && !debugEnabled: // enable debug
		// 		debug.Enable()
		// 	}
		// }
	}

	if err := config.Reload(*cli.configFile, cli.flags, reload); err != nil {
		logrus.Error(err)
	}
}

func (cli *DaemonCli) stop() {
	cli.api.Close()
}

// shutdownDaemon just wraps daemon.Shutdown() to handle a timeout in case
// d.Shutdown() is waiting too long to kill container or worst it's
// blocked there
func shutdownDaemon(d *daemon.Daemon) {
	shutdownTimeout := d.ShutdownTimeout()
	ch := make(chan struct{})
	go func() {
		d.Shutdown()
		close(ch)
	}()
	if shutdownTimeout < 0 {
		<-ch
		logrus.Debug("Clean shutdown succeeded")
		return
	}
	select {
	case <-ch:
		logrus.Debug("Clean shutdown succeeded")
	case <-time.After(time.Duration(shutdownTimeout) * time.Second):
		logrus.Error("Force shutdown daemon")
	}
}

func loadDaemonCliConfig(opts *daemonOptions) (*config.Config, error) {
	conf := opts.daemonConfig
	flags := opts.flags
	conf.Debug = opts.Debug
	conf.Hosts = opts.Hosts
	conf.LogLevel = opts.LogLevel
	// conf.TLS = opts.TLS
	// conf.TLSVerify = opts.TLSVerify
	// conf.CommonTLSOptions = config.CommonTLSOptions{}

	// if opts.TLSOptions != nil {
	// 	conf.CommonTLSOptions.CAFile = opts.TLSOptions.CAFile
	// 	conf.CommonTLSOptions.CertFile = opts.TLSOptions.CertFile
	// 	conf.CommonTLSOptions.KeyFile = opts.TLSOptions.KeyFile
	// }

	// if conf.TrustKeyPath == "" {
	// 	conf.TrustKeyPath = filepath.Join(
	// 		getDaemonConfDir(conf.Root),
	// 		defaultTrustKeyFile)
	// }

	// if flags.Changed("graph") && flags.Changed("data-root") {
	// 	return nil, fmt.Errorf(`cannot specify both "--graph" and "--data-root" option`)
	// }

	if opts.configFile != "" {
		c, err := config.MergeDaemonConfigurations(conf, flags, opts.configFile)
		if err != nil {
			if flags.Changed("config-file") || !os.IsNotExist(err) {
				return nil, fmt.Errorf("unable to configure the Docker daemon with file %s: %v", opts.configFile, err)
			}
		}
		// the merged configuration can be nil if the config file didn't exist.
		// leave the current configuration as it is if when that happens.
		if c != nil {
			conf = c
		}
	}

	if err := config.Validate(conf); err != nil {
		return nil, err
	}

	// ensure that the log level is the one set after merging configurations
	setLogLevel(conf.LogLevel)

	return conf, nil
}

func initRouter(opts routerOptions) {

	routers := []router.Router{
		pluginrouter.NewRouter(opts.daemon.PluginManager()),
	}

	// if opts.daemon.NetworkControllerEnabled() {
	// 	routers = append(routers, network.NewRouter(opts.daemon, opts.cluster))
	// }

	opts.api.InitRouter(routers...)
}

// TODO: remove this from cli and return the authzMiddleware
func (cli *DaemonCli) initMiddlewares(s *apiserver.Server, cfg *apiserver.Config) error {
	// v := cfg.Version

	// exp := middleware.NewExperimentalMiddleware(cli.Config.Experimental)
	// s.UseMiddleware(exp)

	// vm := middleware.NewVersionMiddleware(v, api.DefaultVersion, api.MinVersion)
	// s.UseMiddleware(vm)

	// if cfg.CorsHeaders != "" {
	// 	c := middleware.NewCORSMiddleware(cfg.CorsHeaders)
	// 	s.UseMiddleware(c)
	// }

	// cli.authzMiddleware = authorization.NewMiddleware(cli.Config.AuthorizationPlugins, pluginStore)
	// cli.Config.AuthzMiddleware = cli.authzMiddleware
	// s.UseMiddleware(cli.authzMiddleware)
	return nil
}

func newAPIServerConfig(cli *DaemonCli) (*apiserver.Config, error) {
	serverConfig := &apiserver.Config{
		Logging:     true,
		SocketGroup: cli.Config.SocketGroup,
		Version:     dockerversion.Version,
		CorsHeaders: cli.Config.CorsHeaders,
	}

	// if cli.Config.TLS {
	// 	tlsOptions := tlsconfig.Options{
	// 		CAFile:             cli.Config.CommonTLSOptions.CAFile,
	// 		CertFile:           cli.Config.CommonTLSOptions.CertFile,
	// 		KeyFile:            cli.Config.CommonTLSOptions.KeyFile,
	// 		ExclusiveRootPools: true,
	// 	}

	// 	if cli.Config.TLSVerify {
	// 		// server requires and verifies client's certificate
	// 		tlsOptions.ClientAuth = tls.RequireAndVerifyClientCert
	// 	}
	// 	tlsConfig, err := tlsconfig.Server(tlsOptions)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// 	serverConfig.TLSConfig = tlsConfig
	// }

	if len(cli.Config.Hosts) == 0 {
		cli.Config.Hosts = make([]string, 1)
	}

	return serverConfig, nil
}

func loadListeners(cli *DaemonCli, serverConfig *apiserver.Config) ([]string, error) {
	var hosts []string
	for i := 0; i < len(cli.Config.Hosts); i++ {
		var err error
		if cli.Config.Hosts[i], err = dopts.ParseHost(cli.Config.TLS, cli.Config.Hosts[i]); err != nil {
			return nil, fmt.Errorf("error parsing -H %s : %v", cli.Config.Hosts[i], err)
		}

		protoAddr := cli.Config.Hosts[i]
		protoAddrParts := strings.SplitN(protoAddr, "://", 2)
		if len(protoAddrParts) != 2 {
			return nil, fmt.Errorf("bad format %s, expected PROTO://ADDR", protoAddr)
		}

		proto := protoAddrParts[0]
		addr := protoAddrParts[1]

		// It's a bad idea to bind to TCP without tlsverify.
		// if proto == "tcp" && (serverConfig.TLSConfig == nil || serverConfig.TLSConfig.ClientAuth != tls.RequireAndVerifyClientCert) {
		// 	logrus.Warn("[!] DON'T BIND ON ANY IP ADDRESS WITHOUT setting --tlsverify IF YOU DON'T KNOW WHAT YOU'RE DOING [!]")
		// }
		ls, err := listeners.Init(proto, addr, serverConfig.SocketGroup, serverConfig.TLSConfig)
		if err != nil {
			return nil, err
		}
		ls = wrapListeners(proto, ls)
		// If we're binding to a TCP port, make sure that a container doesn't try to use it.
		if proto == "tcp" {
			if err := allocateDaemonPort(addr); err != nil {
				return nil, err
			}
		}
		logrus.Debugf("Listener created for HTTP on %s (%s)", proto, addr)
		hosts = append(hosts, protoAddrParts[1])
		cli.api.Accept(addr, ls...)
	}

	return hosts, nil
}
