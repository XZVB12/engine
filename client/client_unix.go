// +build linux freebsd openbsd darwin

package client

// DefaultDockerHost defines os specific default if DOCKER_HOST is unset
const DefaultDockerHost = "unix:///var/run/malice.sock"

const defaultProto = "unix"
const defaultAddr = "/var/run/malice.sock"
