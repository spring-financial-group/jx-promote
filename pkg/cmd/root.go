package cmd

import (
	"github.com/jenkins-x-plugins/jx-promote/pkg/multipromote"
	"github.com/jenkins-x-plugins/jx-promote/pkg/promote"
	"github.com/spf13/cobra"
)

// Main creates a command object for the command
func Main() (*cobra.Command, *promote.Options) {
	cmd, opts := promote.NewCmdPromote()
	cmd.AddCommand(multipromote.NewCmdMultiplePromote())
	return cmd, opts
}
