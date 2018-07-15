package commands

import (
	"github.com/maliceio/engine/cli/malice/command"
	"github.com/maliceio/engine/cli/malice/command/plugin"
	"github.com/maliceio/engine/cli/malice/command/scan"
	"github.com/maliceio/engine/cli/malice/command/search"
	"github.com/maliceio/engine/cli/malice/command/swarm"
	"github.com/maliceio/engine/cli/malice/command/watch"
	"github.com/maliceio/engine/cli/malice/command/web"
	"github.com/spf13/cobra"
)

// AddCommands adds all the commands from cli/command to the root command
func AddCommands(cmd *cobra.Command, maliceCli *command.MaliceCli) {
	cmd.AddCommand(
		// plugin
		plugin.NewPluginCommand(maliceCli),

		// swarm
		swarm.NewSwarmCommand(maliceCli),

		// watch
		watch.NewWatchCommand(maliceCli),

		// web
		web.NewWebCommand(maliceCli),

		// scan
		scan.NewScanCommand(maliceCli),

		// search
		search.NewSearchCommand(maliceCli),
	)

}
