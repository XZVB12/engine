package web

import (
	"github.com/maliceio/cli/cli"
	"github.com/maliceio/cli/cli/command"
	"github.com/spf13/cobra"
)

// NewWebCommand returns a cobra command for `web` subcommands
// nolint: interfacer
func NewWebCommand(maliceCli *command.MaliceCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Manage web services",
		Args:  cli.NoArgs,
		RunE:  command.ShowHelp(maliceCli.Err()),
	}

	cmd.AddCommand(
		newStartCommand(maliceCli),
		newStopCommand(maliceCli),
		newBackUpCommand(maliceCli),
	)
	return cmd
}
