package types

import (
	"github.com/maliceio/engine/api/types/registry"
)

// Version contains response of Engine API:
// GET "/version"
type Version struct {
	Version       string
	APIVersion    string `json:"ApiVersion"`
	MinAPIVersion string `json:"MinAPIVersion,omitempty"`
	GitCommit     string
	GoVersion     string
	Os            string
	Arch          string
	KernelVersion string `json:",omitempty"`
	Experimental  bool   `json:",omitempty"`
	BuildTime     string `json:",omitempty"`
}

// ErrorResponse Represents an error.
type ErrorResponse struct {

	// The error message.
	// Required: true
	Message string `json:"message"`
}

// Ping contains response of Engine API:
// GET "/_ping"
type Ping struct {
	APIVersion string
	OSType     string
}

// DiskUsage contains response of Engine API:
// GET "/system/df"
type DiskUsage struct {
	LayersSize int64
	// Images      []*ImageSummary
	// Containers  []*Container
	// Volumes     []*Volume
	BuilderSize int64
}

// Info contains response of Engine API:
// GET "/info"
type Info struct {
	ID                string
	Containers        int
	ContainersRunning int
	ContainersPaused  int
	ContainersStopped int
	Images            int
	Driver            string
	DriverStatus      [][2]string
	SystemStatus      [][2]string
	// Plugins            PluginsInfo
	MemoryLimit        bool
	SwapLimit          bool
	KernelMemory       bool
	CPUCfsPeriod       bool `json:"CpuCfsPeriod"`
	CPUCfsQuota        bool `json:"CpuCfsQuota"`
	CPUShares          bool
	CPUSet             bool
	IPv4Forwarding     bool
	BridgeNfIptables   bool
	BridgeNfIP6tables  bool `json:"BridgeNfIp6tables"`
	Debug              bool
	NFd                int
	OomKillDisable     bool
	NGoroutines        int
	SystemTime         string
	LoggingDriver      string
	CgroupDriver       string
	NEventsListener    int
	KernelVersion      string
	OperatingSystem    string
	OSType             string
	Architecture       string
	IndexServerAddress string
	RegistryConfig     *registry.ServiceConfig
	NCPU               int
	MemTotal           int64
	// GenericResources   []swarm.GenericResource
	DockerRootDir     string
	HTTPProxy         string `json:"HttpProxy"`
	HTTPSProxy        string `json:"HttpsProxy"`
	NoProxy           string
	Name              string
	Labels            []string
	ExperimentalBuild bool
	ServerVersion     string
	ClusterStore      string
	ClusterAdvertise  string
	// Runtimes           map[string]Runtime
	DefaultRuntime string
	// Swarm          swarm.Info
	// LiveRestoreEnabled determines whether containers should be kept
	// running when the daemon is shutdown or upon daemon start if
	// running containers are detected
	LiveRestoreEnabled bool
	// Isolation          container.Isolation
	InitBinary string
	// ContainerdCommit   Commit
	// RuncCommit         Commit
	// InitCommit         Commit
	SecurityOptions []string
}
