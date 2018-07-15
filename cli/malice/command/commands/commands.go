package commands

import (
	"github.com/maliceio/cli/cli/command"
	"github.com/maliceio/cli/cli/command/plugin"
	"github.com/maliceio/cli/cli/command/scan"
	"github.com/maliceio/cli/cli/command/search"
	"github.com/maliceio/cli/cli/command/swarm"
	"github.com/maliceio/cli/cli/command/watch"
	"github.com/maliceio/cli/cli/command/web"
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
