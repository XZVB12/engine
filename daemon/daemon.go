// Package daemon exposes the functions that occur on the host server
// that the Malice daemon is running.
package daemon

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	containerd "github.com/containerd/containerd/api/grpc/types"
	"github.com/maliceio/engine/api/types"
	"github.com/maliceio/engine/api/types/swarm"
	"github.com/maliceio/engine/daemon/config"
	"github.com/maliceio/engine/daemon/events"
	"github.com/maliceio/engine/daemon/logger"
	"github.com/sirupsen/logrus"

	"github.com/docker/docker/pkg/idtools"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/sysinfo"
	"github.com/docker/docker/pkg/system"
	"github.com/docker/docker/pkg/truncindex"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/cluster"
	nwconfig "github.com/docker/libnetwork/config"
	"github.com/docker/libtrust"
	"github.com/maliceio/engine/malice/version"
	"github.com/maliceio/engine/plugin"
	"github.com/maliceio/engine/registry"
	refstore "github.com/maliceio/engine/registry/reference"
	"github.com/pkg/errors"
)

var errSystemNotSupported = errors.New("the Malice daemon is not supported on this platform")

type daemonStore struct {
	graphDriver string
	imageRoot   string
	// imageStore                image.Store
	// layerStore                layer.Store
	// distributionMetadataStore dmetadata.Store
}

// Daemon holds information about the Malice daemon.
type Daemon struct {
	ID         string
	repository string

	// execCommands *exec.Store

	trustKey    libtrust.PrivateKey
	idIndex     *truncindex.TruncIndex
	configStore *config.Config
	// statsCollector   *stats.Collector
	RegistryService registry.Service
	EventsService   *events.Events
	netController   libnetwork.NetworkController
	// volumes          *store.VolumeStore
	// discoveryWatcher discovery.Reloader
	root            string
	seccompEnabled  bool
	apparmorEnabled bool
	shutdown        bool
	idMappings      *idtools.IDMappings
	stores          map[string]daemonStore // By plugin target platform
	referenceStore  refstore.Store
	PluginStore     *plugin.Store // todo: remove
	pluginManager   *plugin.Manager
	linkIndex       *linkIndex

	clusterProvider       cluster.Provider
	cluster               Cluster
	genericResources      []swarm.GenericResource
	metricsPluginListener net.Listener

	machineMemory uint64

	diskUsageRunning int32
	pruneRunning     int32
	hosts            map[string]bool // hosts stores the addresses the daemon is listening on
	startupDone      chan struct{}
}

// StoreHosts stores the addresses the daemon is listening on
func (daemon *Daemon) StoreHosts(hosts []string) {
	if daemon.hosts == nil {
		daemon.hosts = make(map[string]bool)
	}
	for _, h := range hosts {
		daemon.hosts[h] = true
	}
}

func (daemon *Daemon) restore() error {
	plugins := make(map[string]*plugin.Plugin)

	logrus.Info("Loading plugins: start.")

	dir, err := ioutil.ReadDir(daemon.repository)
	if err != nil {
		return err
	}

	for _, v := range dir {
		id := v.Name()
		plugin, err := daemon.load(id)
		if err != nil {
			logrus.Errorf("Failed to load plugin %v: %v", id, err)
			continue
		}

		// Ignore the plugin if it does not support the current driver being used by the graph
		currentDriverForContainerPlatform := daemon.stores[plugin.Platform].graphDriver
		if (plugin.Driver == "" && currentDriverForContainerPlatform == "aufs") || plugin.Driver == currentDriverForContainerPlatform {
			rwlayer, err := daemon.stores[plugin.Platform].layerStore.GetRWLayer(plugin.ID)
			if err != nil {
				logrus.Errorf("Failed to load plugin mount %v: %v", id, err)
				continue
			}
			plugin.RWLayer = rwlayer
			logrus.Debugf("Loaded plugin %v", plugin.ID)

			containers[plugin.ID] = plugin
		} else {
			logrus.Debugf("Cannot load plugin %s because it was created with another graph driver.", plugin.ID)
		}
	}

	removePlugins := make(map[string]*plugin.Plugin)
	restartPlugins := make(map[*plugin.Plugin]chan struct{})
	activeSandboxes := make(map[string]interface{})
	for id, p := range containers {
		if err := daemon.registerName(p); err != nil {
			logrus.Errorf("Failed to register plugin name %s: %s", p.ID, err)
			delete(containers, id)
			continue
		}
		// verify that all volumes valid and have been migrated from the pre-1.7 layout
		if err := daemon.verifyVolumesInfo(p); err != nil {
			// don't skip the plugin due to error
			logrus.Errorf("Failed to verify volumes for plugin '%s': %v", p.ID, err)
		}
		if err := daemon.Register(p); err != nil {
			logrus.Errorf("Failed to register plugin %s: %s", p.ID, err)
			delete(containers, id)
			continue
		}

		// The LogConfig.Type is empty if the plugin was created before malice 1.12 with default log driver.
		// We should rewrite it to use the daemon defaults.
		// Fixes https://github.com/docker/docker/issues/22536
		if p.HostConfig.LogConfig.Type == "" {
			if err := daemon.mergeAndVerifyLogConfig(&p.HostConfig.LogConfig); err != nil {
				logrus.Errorf("Failed to verify log config for plugin %s: %q", p.ID, err)
				continue
			}
		}
	}

	var wg sync.WaitGroup
	var mapLock sync.Mutex
	for _, p := range plugins {
		wg.Add(1)
		go func(p *plugin.Plugin) {
			defer wg.Done()
			daemon.backportMountSpec(p)
			if err := daemon.checkpointAndSave(p); err != nil {
				logrus.WithError(err).WithField("plugin", p.ID).Error("error saving backported mountspec to disk")
			}

			daemon.setStateCounter(p)
			if p.IsRunning() || p.IsPaused() {
				p.RestartManager().Cancel() // manually start containers because some need to wait for swarm networking
				if err := daemon.containerd.Restore(p.ID, p.InitializeStdio); err != nil {
					logrus.Errorf("Failed to restore %s with containerd: %s", p.ID, err)
					return
				}

				// we call Mount and then Unmount to get BaseFs of the plugin
				if err := daemon.Mount(p); err != nil {
					// The mount is unlikely to fail. However, in case mount fails
					// the plugin should be allowed to restore here. Some functionalities
					// (like malice exec -u user) might be missing but plugin is able to be
					// stopped/restarted/removed.
					// See #29365 for related information.
					// The error is only logged here.
					logrus.Warnf("Failed to mount plugin on getting BaseFs path %v: %v", p.ID, err)
				} else {
					if err := daemon.Unmount(p); err != nil {
						logrus.Warnf("Failed to umount plugin on getting BaseFs path %v: %v", p.ID, err)
					}
				}

				p.ResetRestartManager(false)
				if !p.HostConfig.NetworkMode.IsContainer() && p.IsRunning() {
					options, err := daemon.buildSandboxOptions(p)
					if err != nil {
						logrus.Warnf("Failed build sandbox option to restore plugin %s: %v", p.ID, err)
					}
					mapLock.Lock()
					activeSandboxes[p.NetworkSettings.SandboxID] = options
					mapLock.Unlock()
				}

			}
			// fixme: only if not running
			// get list of containers we need to restart
			if !p.IsRunning() && !p.IsPaused() {
				// Do not autostart containers which
				// has endpoints in a swarm scope
				// network yet since the cluster is
				// not initialized yet. We will start
				// it after the cluster is
				// initialized.
				if daemon.configStore.AutoRestart && p.ShouldRestart() && !p.NetworkSettings.HasSwarmEndpoint {
					mapLock.Lock()
					restartContainers[p] = make(chan struct{})
					mapLock.Unlock()
				} else if p.HostConfig != nil && p.HostConfig.AutoRemove {
					mapLock.Lock()
					removeContainers[p.ID] = p
					mapLock.Unlock()
				}
			}

			p.Lock()
			if p.RemovalInProgress {
				// We probably crashed in the middle of a removal, reset
				// the flag.
				//
				// We DO NOT remove the plugin here as we do not
				// know if the user had requested for either the
				// associated volumes, network links or both to also
				// be removed. So we put the plugin in the "dead"
				// state and leave further processing up to them.
				logrus.Debugf("Resetting RemovalInProgress flag from %v", p.ID)
				p.RemovalInProgress = false
				p.Dead = true
				if err := p.CheckpointTo(daemon.containersReplica); err != nil {
					logrus.Errorf("Failed to update plugin %s state: %v", p.ID, err)
				}
			}
			p.Unlock()
		}(p)
	}
	wg.Wait()
	daemon.netController, err = daemon.initNetworkController(daemon.configStore, activeSandboxes)
	if err != nil {
		return fmt.Errorf("Error initializing network controller: %v", err)
	}

	// Now that all the containers are registered, register the links
	for _, p := range containers {
		if err := daemon.registerLinks(p, p.HostConfig); err != nil {
			logrus.Errorf("failed to register link for plugin %s: %v", p.ID, err)
		}
	}

	group := sync.WaitGroup{}
	for p, notifier := range restartContainers {
		group.Add(1)

		go func(p *plugin.Plugin, chNotify chan struct{}) {
			defer group.Done()

			logrus.Debugf("Starting plugin %s", p.ID)

			// ignore errors here as this is a best effort to wait for children to be
			//   running before we try to start the plugin
			children := daemon.children(p)
			timeout := time.After(5 * time.Second)
			for _, child := range children {
				if notifier, exists := restartContainers[child]; exists {
					select {
					case <-notifier:
					case <-timeout:
					}
				}
			}

			// Make sure networks are available before starting
			daemon.waitForNetworks(p)
			if err := daemon.containerStart(p, "", "", true); err != nil {
				logrus.Errorf("Failed to start plugin %s: %s", p.ID, err)
			}
			close(chNotify)
		}(p, notifier)

	}
	group.Wait()

	removeGroup := sync.WaitGroup{}
	for id := range removeContainers {
		removeGroup.Add(1)
		go func(cid string) {
			if err := daemon.ContainerRm(cid, &types.ContainerRmConfig{ForceRemove: true, RemoveVolume: true}); err != nil {
				logrus.Errorf("Failed to remove plugin %s: %s", cid, err)
			}
			removeGroup.Done()
		}(id)
	}
	removeGroup.Wait()

	// any containers that were started above would already have had this done,
	// however we need to now prepare the mountpoints for the rest of the containers as well.
	// This shouldn't cause any issue running on the containers that already had this run.
	// This must be run after any containers with a restart policy so that containerized plugins
	// can have a chance to be running before we try to initialize them.
	for _, p := range containers {
		// if the plugin has restart policy, do not
		// prepare the mountpoints since it has been done on restarting.
		// This is to speed up the daemon start when a restart plugin
		// has a volume and the volume driver is not available.
		if _, ok := restartContainers[p]; ok {
			continue
		} else if _, ok := removeContainers[p.ID]; ok {
			// plugin is automatically removed, skip it.
			continue
		}

		group.Add(1)
		go func(p *plugin.Plugin) {
			defer group.Done()
			if err := daemon.prepareMountPoints(p); err != nil {
				logrus.Error(err)
			}
		}(p)
	}

	group.Wait()

	logrus.Info("Loading containers: done.")

	return nil
}

// RestartSwarmContainers restarts any autostart plugin which has a
// swarm endpoint.
func (daemon *Daemon) RestartSwarmContainers() {
	group := sync.WaitGroup{}
	for _, p := range daemon.List() {
		if !p.IsRunning() && !p.IsPaused() {
			// Autostart all the containers which has a
			// swarm endpoint now that the cluster is
			// initialized.
			if daemon.configStore.AutoRestart && p.ShouldRestart() && p.NetworkSettings.HasSwarmEndpoint {
				group.Add(1)
				go func(p *plugin.Plugin) {
					defer group.Done()
					if err := daemon.containerStart(p, "", "", true); err != nil {
						logrus.Error(err)
					}
				}(p)
			}
		}

	}
	group.Wait()
}

// waitForNetworks is used during daemon initialization when starting up containers
// It ensures that all of a plugin's networks are available before the daemon tries to start the plugin.
// In practice it just makes sure the discovery service is available for containers which use a network that require discovery.
func (daemon *Daemon) waitForNetworks(p *plugin.Plugin) {
	if daemon.discoveryWatcher == nil {
		return
	}
	// Make sure if the plugin has a network that requires discovery that the discovery service is available before starting
	for netName := range p.NetworkSettings.Networks {
		// If we get `ErrNoSuchNetwork` here, we can assume that it is due to discovery not being ready
		// Most likely this is because the K/V store used for discovery is in a plugin and needs to be started
		if _, err := daemon.netController.NetworkByName(netName); err != nil {
			if _, ok := err.(libnetwork.ErrNoSuchNetwork); !ok {
				continue
			}
			// use a longish timeout here due to some slowdowns in libnetwork if the k/v store is on anything other than --net=host
			// FIXME: why is this slow???
			logrus.Debugf("Container %s waiting for network to be ready", p.Name)
			select {
			case <-daemon.discoveryWatcher.ReadyCh():
			case <-time.After(60 * time.Second):
			}
			return
		}
	}
}

func (daemon *Daemon) children(p *plugin.Plugin) map[string]*plugin.Plugin {
	return daemon.linkIndex.children(p)
}

// parents returns the names of the parent containers of the plugin
// with the given name.
func (daemon *Daemon) parents(p *plugin.Plugin) map[string]*plugin.Plugin {
	return daemon.linkIndex.parents(p)
}

func (daemon *Daemon) registerLink(parent, child *plugin.Plugin, alias string) error {
	fullName := path.Join(parent.Name, alias)
	if err := daemon.containersReplica.ReserveName(fullName, child.ID); err != nil {
		if err == plugin.ErrNameReserved {
			logrus.Warnf("error registering link for %s, to %s, as alias %s, ignoring: %v", parent.ID, child.ID, alias, err)
			return nil
		}
		return err
	}
	daemon.linkIndex.link(parent, child, fullName)
	return nil
}

// DaemonJoinsCluster informs the daemon has joined the cluster and provides
// the handler to query the cluster component
func (daemon *Daemon) DaemonJoinsCluster(clusterProvider cluster.Provider) {
	daemon.setClusterProvider(clusterProvider)
}

// DaemonLeavesCluster informs the daemon has left the cluster
func (daemon *Daemon) DaemonLeavesCluster() {
	// Daemon is in charge of removing the attachable networks with
	// connected containers when the node leaves the swarm
	daemon.clearAttachableNetworks()
	// We no longer need the cluster provider, stop it now so that
	// the network agent will stop listening to cluster events.
	daemon.setClusterProvider(nil)
	// Wait for the networking cluster agent to stop
	daemon.netController.AgentStopWait()
	// Daemon is in charge of removing the ingress network when the
	// node leaves the swarm. Wait for job to be done or timeout.
	// This is called also on graceful daemon shutdown. We need to
	// wait, because the ingress release has to happen before the
	// network controller is stopped.
	if done, err := daemon.ReleaseIngress(); err == nil {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			logrus.Warnf("timeout while waiting for ingress network removal")
		}
	} else {
		logrus.Warnf("failed to initiate ingress network removal: %v", err)
	}
}

// setClusterProvider sets a component for querying the current cluster state.
func (daemon *Daemon) setClusterProvider(clusterProvider cluster.Provider) {
	daemon.clusterProvider = clusterProvider
	daemon.netController.SetClusterProvider(clusterProvider)
}

// IsSwarmCompatible verifies if the current daemon
// configuration is compatible with the swarm mode
func (daemon *Daemon) IsSwarmCompatible() error {
	if daemon.configStore == nil {
		return nil
	}
	return daemon.configStore.IsSwarmCompatible()
}

// NewDaemon sets up everything for the daemon to be able to service
// requests from the webserver.
func NewDaemon(config *config.Config, registryService registry.Service, pluginStore *plugin.Store) (daemon *Daemon, err error) {
	setDefaultMtu(config)

	// Ensure that we have a correct root key limit for launching containers.
	if err := ModifyRootKeyLimit(); err != nil {
		logrus.Warnf("unable to modify root key limit, number of containers could be limited by this quota: %v", err)
	}

	// Ensure we have compatible and valid configuration options
	if err := verifyDaemonSettings(config); err != nil {
		return nil, err
	}

	// Do we have a disabled network?
	config.DisableBridge = isBridgeNetworkDisabled(config)

	// Verify the platform is supported as a daemon
	if !platformSupported {
		return nil, errSystemNotSupported
	}

	// Validate platform-specific requirements
	if err := checkSystem(); err != nil {
		return nil, err
	}

	idMappings, err := setupRemappedRoot(config)
	if err != nil {
		return nil, err
	}
	rootIDs := idMappings.RootPair()
	if err := setupDaemonProcess(config); err != nil {
		return nil, err
	}

	// set up the tmpDir to use a canonical path
	tmp, err := prepareTempDir(config.Root, rootIDs)
	if err != nil {
		return nil, fmt.Errorf("Unable to get the TempDir under %s: %s", config.Root, err)
	}
	realTmp, err := getRealPath(tmp)
	if err != nil {
		return nil, fmt.Errorf("Unable to get the full path to the TempDir (%s): %s", tmp, err)
	}
	os.Setenv("TMPDIR", realTmp)

	d := &Daemon{
		configStore: config,
		startupDone: make(chan struct{}),
	}
	// Ensure the daemon is properly shutdown if there is a failure during
	// initialization
	defer func() {
		if err != nil {
			if err := d.Shutdown(); err != nil {
				logrus.Error(err)
			}
		}
	}()

	if err := d.setGenericResources(config); err != nil {
		return nil, err
	}
	// set up SIGUSR1 handler on Unix-like systems, or a Win32 global event
	// on Windows to dump Go routine stacks
	stackDumpDir := config.Root
	if execRoot := config.GetExecRoot(); execRoot != "" {
		stackDumpDir = execRoot
	}
	d.setupDumpStackTrap(stackDumpDir)

	if err := d.setupSeccompProfile(); err != nil {
		return nil, err
	}

	// Set the default isolation mode (only applicable on Windows)
	if err := d.setDefaultIsolation(); err != nil {
		return nil, fmt.Errorf("error setting default isolation mode: %v", err)
	}

	logrus.Debugf("Using default logging driver %s", config.LogConfig.Type)

	if err := configureMaxThreads(config); err != nil {
		logrus.Warnf("Failed to configure golang's threads limit: %v", err)
	}

	if err := ensureDefaultAppArmorProfile(); err != nil {
		logrus.Errorf(err.Error())
	}

	daemonRepo := filepath.Join(config.Root, "containers")
	if err := idtools.MkdirAllAndChown(daemonRepo, 0700, rootIDs); err != nil && !os.IsExist(err) {
		return nil, err
	}

	if runtime.GOOS == "windows" {
		if err := system.MkdirAll(filepath.Join(config.Root, "credentialspecs"), 0, ""); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}

	// On Windows we don't support the environment variable, or a user supplied graphdriver
	// as Windows has no choice in terms of which graphdrivers to use. It's a case of
	// running Windows containers on Windows - windowsfilter, running Linux containers on Windows,
	// lcow. Unix platforms however run a single graphdriver for all containers, and it can
	// be set through an environment variable, a daemon start parameter, or chosen through
	// initialization of the layerstore through driver priority order for example.
	d.stores = make(map[string]daemonStore)
	if runtime.GOOS == "windows" {
		d.stores["windows"] = daemonStore{graphDriver: "windowsfilter"}
		if system.LCOWSupported() {
			d.stores["linux"] = daemonStore{graphDriver: "lcow"}
		}
	} else {
		driverName := os.Getenv("MALICE_DRIVER")
		if driverName == "" {
			driverName = config.GraphDriver
		} else {
			logrus.Infof("Setting the storage driver from the $MALICE_DRIVER environment variable (%s)", driverName)
		}
		d.stores[runtime.GOOS] = daemonStore{graphDriver: driverName} // May still be empty. Layerstore init determines instead.
	}

	d.RegistryService = registryService
	d.PluginStore = pluginStore
	logger.RegisterPluginGetter(d.PluginStore)

	metricsSockPath, err := d.listenMetricsSock()
	if err != nil {
		return nil, err
	}
	registerMetricsPluginCallback(d.PluginStore, metricsSockPath)

	// Plugin system initialization should happen before restore. Do not change order.
	d.pluginManager, err = plugin.NewManager(plugin.ManagerConfig{
		Root:               filepath.Join(config.Root, "plugins"),
		ExecRoot:           getPluginExecRoot(config.Root),
		Store:              d.PluginStore,
		Executor:           containerdRemote,
		RegistryService:    registryService,
		LiveRestoreEnabled: config.LiveRestoreEnabled,
		LogPluginEvent:     d.LogPluginEvent, // todo: make private
		AuthzMiddleware:    config.AuthzMiddleware,
	})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create plugin manager")
	}

	var graphDrivers []string
	for platform, ds := range d.stores {
		// ls, err := layer.NewStoreFromOptions(layer.StoreOptions{
		// 	StorePath: config.Root,
		// 	// MetadataStorePathTemplate: filepath.Join(config.Root, "image", "%s", "layerdb"),
		// 	GraphDriver:         ds.graphDriver,
		// 	GraphDriverOptions:  config.GraphOptions,
		// 	IDMappings:          idMappings,
		// 	PluginGetter:        d.PluginStore,
		// 	ExperimentalEnabled: config.Experimental,
		// 	Platform:            platform,
		// })
		// if err != nil {
		// 	return nil, err
		// }
		// ds.graphDriver = ls.DriverName() // As layerstore may set the driver
		// ds.layerStore = ls
		d.stores[platform] = ds
		graphDrivers = append(graphDrivers, ls.DriverName())
	}

	// Configure and validate the kernels security support
	if err := configureKernelSecuritySupport(config, graphDrivers); err != nil {
		return nil, err
	}

	logrus.Debugf("Max Concurrent Downloads: %d", *config.MaxConcurrentDownloads)
	// lsMap := make(map[string]layer.Store)
	// for platform, ds := range d.stores {
	// 	lsMap[platform] = ds.layerStore
	// }
	// d.downloadManager = xfer.NewLayerDownloadManager(lsMap, *config.MaxConcurrentDownloads)
	// logrus.Debugf("Max Concurrent Uploads: %d", *config.MaxConcurrentUploads)
	// d.uploadManager = xfer.NewLayerUploadManager(*config.MaxConcurrentUploads)
	for platform, ds := range d.stores {
		// imageRoot := filepath.Join(config.Root, "image", ds.graphDriver)
		// ifs, err := image.NewFSStoreBackend(filepath.Join(imageRoot, "imagedb"))
		// if err != nil {
		// 	return nil, err
		// }

		// var is image.Store
		// is, err = image.NewImageStore(ifs, platform, ds.layerStore)
		// if err != nil {
		// 	return nil, err
		// }
		// ds.imageRoot = imageRoot
		// ds.imageStore = is
		d.stores[platform] = ds
	}

	// Configure the volumes driver
	volStore, err := d.configureVolumes(rootIDs)
	if err != nil {
		return nil, err
	}

	trustKey, err := loadOrCreateTrustKey(config.TrustKeyPath)
	if err != nil {
		return nil, err
	}

	trustDir := filepath.Join(config.Root, "trust")

	if err := system.MkdirAll(trustDir, 0700, ""); err != nil {
		return nil, err
	}

	eventsService := events.New()

	// We have a single tag/reference store for the daemon globally. However, it's
	// stored under the graphdriver. On host platforms which only support a single
	// plugin OS, but multiple selectable graphdrivers, this means depending on which
	// graphdriver is chosen, the global reference store is under there. For
	// platforms which support multiple plugin operating systems, this is slightly
	// more problematic as where does the global ref store get located? Fortunately,
	// for Windows, which is currently the only daemon supporting multiple plugin
	// operating systems, the list of graphdrivers available isn't user configurable.
	// For backwards compatibility, we just put it under the windowsfilter
	// directory regardless.
	refStoreLocation := filepath.Join(d.stores[runtime.GOOS].imageRoot, `repositories.json`)
	rs, err := refstore.NewReferenceStore(refStoreLocation)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create reference store repository: %s", err)
	}
	d.referenceStore = rs

	for platform, ds := range d.stores {
		// dms, err := dmetadata.NewFSMetadataStore(filepath.Join(ds.imageRoot, "distribution"), platform)
		// if err != nil {
		// 	return nil, err
		// }

		// ds.distributionMetadataStore = dms
		d.stores[platform] = ds

	}

	// Discovery is only enabled when the daemon is launched with an address to advertise.  When
	// initialized, the daemon is registered and we can store the discovery backend as it's read-only
	if err := d.initDiscovery(config); err != nil {
		return nil, err
	}

	sysInfo := sysinfo.New(false)
	// Check if Devices cgroup is mounted, it is hard requirement for plugin security,
	// on Linux.
	if runtime.GOOS == "linux" && !sysInfo.CgroupDevicesEnabled {
		return nil, errors.New("Devices cgroup isn't mounted")
	}

	d.ID = trustKey.PublicKey().KeyID()
	d.repository = daemonRepo
	d.containers = plugin.NewMemoryStore()
	if d.containersReplica, err = plugin.NewViewDB(); err != nil {
		return nil, err
	}
	// d.execCommands = exec.NewStore()
	d.trustKey = trustKey
	d.idIndex = truncindex.NewTruncIndex([]string{})
	d.statsCollector = d.newStatsCollector(1 * time.Second)
	d.EventsService = eventsService
	d.volumes = volStore
	d.root = config.Root
	d.idMappings = idMappings
	d.seccompEnabled = sysInfo.Seccomp
	d.apparmorEnabled = sysInfo.AppArmor

	d.linkIndex = newLinkIndex()
	d.containerdRemote = containerdRemote

	go d.execCommandGC()

	d.containerd, err = containerdRemote.Client(d)
	if err != nil {
		return nil, err
	}

	if err := d.restore(); err != nil {
		return nil, err
	}
	close(d.startupDone)

	// FIXME: this method never returns an error
	info, _ := d.SystemInfo()

	engineInfo.WithValues(
		version.Version,
		version.GitCommit,
		info.Architecture,
		info.Driver,
		info.KernelVersion,
		info.OperatingSystem,
		info.OSType,
		info.ID,
	).Set(1)
	engineCpus.Set(float64(info.NCPU))
	engineMemory.Set(float64(info.MemTotal))

	gd := ""
	for platform, ds := range d.stores {
		if len(gd) > 0 {
			gd += ", "
		}
		gd += ds.graphDriver
		if len(d.stores) > 1 {
			gd = fmt.Sprintf("%s (%s)", gd, platform)
		}
	}
	logrus.WithFields(logrus.Fields{
		"version": version.Version,
		"commit":  version.GitCommit,
	}).Info("Malice daemon")

	return d, nil
}

func (daemon *Daemon) waitForStartupDone() {
	<-daemon.startupDone
}

func (daemon *Daemon) shutdownContainer(p *plugin.Plugin) error {
	stopTimeout := p.StopTimeout()

	// If plugin failed to exit in stopTimeout seconds of SIGTERM, then using the force
	if err := daemon.containerStop(p, stopTimeout); err != nil {
		return fmt.Errorf("Failed to stop plugin %s with error: %v", p.ID, err)
	}

	// Wait without timeout for the plugin to exit.
	// Ignore the result.
	<-p.Wait(context.Background(), plugin.WaitConditionNotRunning)
	return nil
}

// ShutdownTimeout returns the shutdown timeout based on the max stopTimeout of the plugins,
// and is limited by daemon's ShutdownTimeout.
func (daemon *Daemon) ShutdownTimeout() int {
	// By default we use daemon's ShutdownTimeout.
	shutdownTimeout := daemon.configStore.ShutdownTimeout

	graceTimeout := 5
	if daemon.plugins != nil {
		for _, p := range daemon.plugins.List() {
			if shutdownTimeout >= 0 {
				stopTimeout := p.StopTimeout()
				if stopTimeout < 0 {
					shutdownTimeout = -1
				} else {
					if stopTimeout+graceTimeout > shutdownTimeout {
						shutdownTimeout = stopTimeout + graceTimeout
					}
				}
			}
		}
	}
	return shutdownTimeout
}

// Shutdown stops the daemon.
func (daemon *Daemon) Shutdown() error {
	daemon.shutdown = true
	// Keep mounts and networking running on daemon shutdown if
	// we are to keep containers running and restore them.

	if daemon.configStore.LiveRestoreEnabled && daemon.containers != nil {
		// check if there are any running containers, if none we should do some cleanup
		if ls, err := daemon.Containers(&types.ContainerListOptions{}); len(ls) != 0 || err != nil {
			// metrics plugins still need some cleanup
			daemon.cleanupMetricsPlugins()
			return nil
		}
	}

	if daemon.containers != nil {
		logrus.Debugf("start clean shutdown of all containers with a %d seconds timeout...", daemon.configStore.ShutdownTimeout)
		daemon.containers.ApplyAll(func(p *plugin.Plugin) {
			if !p.IsRunning() {
				return
			}
			logrus.Debugf("stopping %s", p.ID)
			if err := daemon.shutdownPlugin(p); err != nil {
				logrus.Errorf("Stop plugin error: %v", err)
				return
			}
			// if mountid, err := daemon.stores[p.Platform].layerStore.GetMountID(p.ID); err == nil {
			// 	daemon.cleanupMountsByID(mountid)
			// }
			logrus.Debugf("plugin stopped %s", p.ID)
		})
	}

	if daemon.volumes != nil {
		if err := daemon.volumes.Shutdown(); err != nil {
			logrus.Errorf("Error shutting down volume store: %v", err)
		}
	}

	// for platform, ds := range daemon.stores {
	// 	if ds.layerStore != nil {
	// 		if err := ds.layerStore.Cleanup(); err != nil {
	// 			logrus.Errorf("Error during layer Store.Cleanup(): %v %s", err, platform)
	// 		}
	// 	}
	// }

	// If we are part of a cluster, clean up cluster's stuff
	if daemon.clusterProvider != nil {
		logrus.Debugf("start clean shutdown of cluster resources...")
		daemon.DaemonLeavesCluster()
	}

	daemon.cleanupMetricsPlugins()

	// Shutdown plugins after containers and layerstore. Don't change the order.
	daemon.pluginShutdown()

	// trigger libnetwork Stop only if it's initialized
	if daemon.netController != nil {
		daemon.netController.Stop()
	}

	if err := daemon.cleanupMounts(); err != nil {
		return err
	}

	return nil
}

// Subnets return the IPv4 and IPv6 subnets of networks that are manager by Malice.
func (daemon *Daemon) Subnets() ([]net.IPNet, []net.IPNet) {
	var v4Subnets []net.IPNet
	var v6Subnets []net.IPNet

	managedNetworks := daemon.netController.Networks()

	for _, managedNetwork := range managedNetworks {
		v4infos, v6infos := managedNetwork.Info().IpamInfo()
		for _, info := range v4infos {
			if info.IPAMData.Pool != nil {
				v4Subnets = append(v4Subnets, *info.IPAMData.Pool)
			}
		}
		for _, info := range v6infos {
			if info.IPAMData.Pool != nil {
				v6Subnets = append(v6Subnets, *info.IPAMData.Pool)
			}
		}
	}

	return v4Subnets, v6Subnets
}

// prepareTempDir prepares and returns the default directory to use
// for temporary files.
// If it doesn't exist, it is created. If it exists, its content is removed.
func prepareTempDir(rootDir string, rootIDs idtools.IDPair) (string, error) {
	var tmpDir string
	if tmpDir = os.Getenv("MALICE_TMPDIR"); tmpDir == "" {
		tmpDir = filepath.Join(rootDir, "tmp")
		newName := tmpDir + "-old"
		if err := os.Rename(tmpDir, newName); err == nil {
			go func() {
				if err := os.RemoveAll(newName); err != nil {
					logrus.Warnf("failed to delete old tmp directory: %s", newName)
				}
			}()
		} else {
			logrus.Warnf("failed to rename %s for background deletion: %s. Deleting synchronously", tmpDir, err)
			if err := os.RemoveAll(tmpDir); err != nil {
				logrus.Warnf("failed to delete old tmp directory: %s", tmpDir)
			}
		}
	}
	// We don't remove the content of tmpdir if it's not the default,
	// it may hold things that do not belong to us.
	return tmpDir, idtools.MkdirAllAndChown(tmpDir, 0700, rootIDs)
}

// func (daemon *Daemon) setupInitLayer(initPath containerfs.ContainerFS) error {
// 	rootIDs := daemon.idMappings.RootPair()
// 	return initlayer.Setup(initPath, rootIDs)
// }

func (daemon *Daemon) setGenericResources(conf *config.Config) error {
	genericResources, err := config.ParseGenericResources(conf.NodeGenericResources)
	if err != nil {
		return err
	}

	daemon.genericResources = genericResources

	return nil
}

func setDefaultMtu(conf *config.Config) {
	// do nothing if the config does not have the default 0 value.
	if conf.Mtu != 0 {
		return
	}
	conf.Mtu = config.DefaultNetworkMtu
}

// func (daemon *Daemon) configureVolumes(rootIDs idtools.IDPair) (*store.VolumeStore, error) {
// 	volumesDriver, err := local.New(daemon.configStore.Root, rootIDs)
// 	if err != nil {
// 		return nil, err
// 	}

// 	volumedrivers.RegisterPluginGetter(daemon.PluginStore)

// 	if !volumedrivers.Register(volumesDriver, volumesDriver.Name()) {
// 		return nil, errors.New("local volume driver could not be registered")
// 	}
// 	return store.New(daemon.configStore.Root)
// }

// IsShuttingDown tells whether the daemon is shutting down or not
func (daemon *Daemon) IsShuttingDown() bool {
	return daemon.shutdown
}

// initDiscovery initializes the discovery watcher for this daemon.
// func (daemon *Daemon) initDiscovery(conf *config.Config) error {
// 	advertise, err := config.ParseClusterAdvertiseSettings(conf.ClusterStore, conf.ClusterAdvertise)
// 	if err != nil {
// 		if err == discovery.ErrDiscoveryDisabled {
// 			return nil
// 		}
// 		return err
// 	}

// 	conf.ClusterAdvertise = advertise
// 	discoveryWatcher, err := discovery.Init(conf.ClusterStore, conf.ClusterAdvertise, conf.ClusterOpts)
// 	if err != nil {
// 		return fmt.Errorf("discovery initialization failed (%v)", err)
// 	}

// 	daemon.discoveryWatcher = discoveryWatcher
// 	return nil
// }

func isBridgeNetworkDisabled(conf *config.Config) bool {
	return conf.BridgeConfig.Iface == config.DisableNetworkBridge
}

func (daemon *Daemon) networkOptions(dconfig *config.Config, pg plugingetter.PluginGetter, activeSandboxes map[string]interface{}) ([]nwconfig.Option, error) {
	options := []nwconfig.Option{}
	if dconfig == nil {
		return options, nil
	}

	options = append(options, nwconfig.OptionExperimental(dconfig.Experimental))
	options = append(options, nwconfig.OptionDataDir(dconfig.Root))
	options = append(options, nwconfig.OptionExecRoot(dconfig.GetExecRoot()))

	// dd := runconfig.DefaultDaemonNetworkMode()
	// dn := runconfig.DefaultDaemonNetworkMode().NetworkName()
	options = append(options, nwconfig.OptionDefaultDriver(string(dd)))
	options = append(options, nwconfig.OptionDefaultNetwork(dn))

	if strings.TrimSpace(dconfig.ClusterStore) != "" {
		kv := strings.Split(dconfig.ClusterStore, "://")
		if len(kv) != 2 {
			return nil, errors.New("kv store daemon config must be of the form KV-PROVIDER://KV-URL")
		}
		options = append(options, nwconfig.OptionKVProvider(kv[0]))
		options = append(options, nwconfig.OptionKVProviderURL(kv[1]))
	}
	if len(dconfig.ClusterOpts) > 0 {
		options = append(options, nwconfig.OptionKVOpts(dconfig.ClusterOpts))
	}

	if daemon.discoveryWatcher != nil {
		options = append(options, nwconfig.OptionDiscoveryWatcher(daemon.discoveryWatcher))
	}

	if dconfig.ClusterAdvertise != "" {
		options = append(options, nwconfig.OptionDiscoveryAddress(dconfig.ClusterAdvertise))
	}

	options = append(options, nwconfig.OptionLabels(dconfig.Labels))
	options = append(options, driverOptions(dconfig)...)

	if daemon.configStore != nil && daemon.configStore.LiveRestoreEnabled && len(activeSandboxes) != 0 {
		options = append(options, nwconfig.OptionActiveSandboxes(activeSandboxes))
	}

	if pg != nil {
		options = append(options, nwconfig.OptionPluginGetter(pg))
	}

	options = append(options, nwconfig.OptionNetworkControlPlaneMTU(dconfig.NetworkControlPlaneMTU))

	return options, nil
}

func copyBlkioEntry(entries []*containerd.BlkioStatsEntry) []types.BlkioStatEntry {
	out := make([]types.BlkioStatEntry, len(entries))
	for i, re := range entries {
		out[i] = types.BlkioStatEntry{
			Major: re.Major,
			Minor: re.Minor,
			Op:    re.Op,
			Value: re.Value,
		}
	}
	return out
}

// GetCluster returns the cluster
func (daemon *Daemon) GetCluster() Cluster {
	return daemon.cluster
}

// SetCluster sets the cluster
func (daemon *Daemon) SetCluster(cluster Cluster) {
	daemon.cluster = cluster
}

func (daemon *Daemon) pluginShutdown() {
	manager := daemon.pluginManager
	// Check for a valid manager object. In error conditions, daemon init can fail
	// and shutdown called, before plugin manager is initialized.
	if manager != nil {
		manager.Shutdown()
	}
}

// PluginManager returns current pluginManager associated with the daemon
func (daemon *Daemon) PluginManager() *plugin.Manager { // set up before daemon to avoid this method
	return daemon.pluginManager
}

// PluginGetter returns current pluginStore associated with the daemon
func (daemon *Daemon) PluginGetter() *plugin.Store {
	return daemon.PluginStore
}

// CreateDaemonRoot creates the root for the daemon
func CreateDaemonRoot(config *config.Config) error {
	// get the canonical path to the Malice root directory
	var realRoot string
	if _, err := os.Stat(config.Root); err != nil && os.IsNotExist(err) {
		realRoot = config.Root
	} else {
		realRoot, err = getRealPath(config.Root)
		if err != nil {
			return fmt.Errorf("Unable to get the full path to root (%s): %s", config.Root, err)
		}
	}

	idMappings, err := setupRemappedRoot(config)
	if err != nil {
		return err
	}
	return setupDaemonRoot(config, realRoot, idMappings.RootPair())
}

// // checkpointAndSave grabs a plugin lock to safely call plugin.CheckpointTo
// func (daemon *Daemon) checkpointAndSave(plugin *plugin.Plugin) error {
// 	plugin.Lock()
// 	defer plugin.Unlock()
// 	if err := plugin.CheckpointTo(daemon.containersReplica); err != nil {
// 		return fmt.Errorf("Error saving plugin state: %v", err)
// 	}
// 	return nil
// }

// because the CLI sends a -1 when it wants to unset the swappiness value
// we need to clear it on the server side
// func fixMemorySwappiness(resources *containertypes.Resources) {
// 	if resources.MemorySwappiness != nil && *resources.MemorySwappiness == -1 {
// 		resources.MemorySwappiness = nil
// 	}
// }
