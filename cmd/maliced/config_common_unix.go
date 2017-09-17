// +build solaris linux freebsd

package main

import (
	"github.com/maliceio/engine/api/types"
	"github.com/maliceio/engine/daemon/config"
	"github.com/maliceio/engine/opts"
	"github.com/spf13/pflag"
)

var (
	defaultPidFile  = "/var/run/malice.pid"
	defaultDataRoot = "/var/lib/malice"
	defaultExecRoot = "/var/run/malice"
)