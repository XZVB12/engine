// Package runtime defines the interface between the GCS and an OCI container
// runtime.
package runtime

import (
	"io"

	"github.com/Microsoft/opengcs/service/gcs/oslayer"
	"github.com/Microsoft/opengcs/service/gcs/stdio"
	oci "github.com/opencontainers/runtime-spec/specs-go"
)

// ContainerState gives information about a container created by a Runtime.
type ContainerState struct {
	OCIVersion string
	ID         string
	Pid        int
	BundlePath string
	RootfsPath string
	Status     string
	Created    string
}

// ContainerProcessState gives information about a process created by a
// Runtime.
type ContainerProcessState struct {
	Pid              int
	Command          []string
	CreatedByRuntime bool
	IsZombie         bool
}

// StdioPipes contain the interfaces for reading from and writing to a
// process's stdio.
type StdioPipes struct {
	In  io.WriteCloser
	Out io.ReadCloser
	Err io.ReadCloser
}

// Process is an interface to manipulate process state.
type Process interface {
	Wait() (oslayer.ProcessExitState, error)
	Pid() int
	Delete() error
	Tty() *stdio.TtyRelay
}

// Container is an interface to manipulate container state.
type Container interface {
	Process
	ID() string
	Exists() (bool, error)
	Start() error
	ExecProcess(process oci.Process, stdioSet *stdio.ConnectionSet) (p Process, err error)
	Kill(signal oslayer.Signal) error
	Pause() error
	Resume() error
	GetState() (*ContainerState, error)
	GetRunningProcesses() ([]ContainerProcessState, error)
	GetAllProcesses() ([]ContainerProcessState, error)
}

// Runtime is the interface defining commands over an OCI container runtime,
// such as runC.
type Runtime interface {
	CreateContainer(id string, bundlePath string, stdioSet *stdio.ConnectionSet) (c Container, err error)
	ListContainerStates() ([]ContainerState, error)
}
