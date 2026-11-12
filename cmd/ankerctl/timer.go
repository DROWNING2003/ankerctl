package main

import (
	"github.com/DROWNING2003/ankerctl/internal/commands"
	"github.com/spf13/cobra"
)

func timerCommand() *cobra.Command {
	root := &cobra.Command{Use: "timer", Short: "Control the dock's built-in Pomodoro timer (experimental)"}
	for _, operation := range []string{"status", "start", "stats", "stop"} {
		root.AddCommand(&cobra.Command{Use: operation, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return commands.Timer(cmd.Context(), opt, operation, "") }})
	}
	root.AddCommand(&cobra.Command{Use: "duration [seconds|duration]", Short: "Read or set duration (30s..1h, multiples of 30s; e.g. 25m)", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		value := ""
		if len(args) == 1 {
			value = args[0]
		}
		return commands.Timer(cmd.Context(), opt, "duration", value)
	}})
	return root
}
