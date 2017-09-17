// +build linux,!solaris freebsd,!solaris

package main

import (
	"github.com/maliceio/engine/daemon/config"
	"github.com/maliceio/engine/opts"
	units "github.com/docker/go-units"
	"github.com/spf13/pflag"
)

// installConfigFlags adds flags to the pflag.FlagSet to configure the daemon
func installConfigFlags(conf *config.Config, flags *pflag.FlagSet) {
	// First handle install flags which are consistent cross-platform
	installCommonConfigFlags(conf, flags)
}
