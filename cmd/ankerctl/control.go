package main

import (
	"fmt"
	"github.com/DROWNING2003/ankerctl/internal/commands"
	"github.com/spf13/cobra"
)

func controlCommand(use, operation string, args cobra.PositionalArgs) *cobra.Command {
	return &cobra.Command{Use: use, Args: args, RunE: func(cmd *cobra.Command, values []string) error {
		value := ""
		if len(values) > 0 {
			value = values[0]
		}
		return commands.Control(cmd.Context(), opt, operation, value)
	}}
}
func reminderCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "reminder", Short: "Read or set the device health reminder interval"}
	cmd.AddCommand(controlCommand("duration [duration]", "reminder", cobra.MaximumNArgs(1)))
	return cmd
}
func focusCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "focus", Short: "Device focus accounting (not application activity tracking)"}
	for _, op := range []string{"status", "on", "off"} {
		cmd.AddCommand(controlCommand(op, "focus-"+op, cobra.NoArgs))
	}
	return cmd
}
func deleteScreenCommand() *cobra.Command {
	yes := false
	cmd := &cobra.Command{Use: "delete <crc>", Short: "Delete one stored wallpaper by hexadecimal CRC", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := commands.ParseCRC(args[0]); err != nil {
			return err
		}
		if !yes {
			return fmt.Errorf("deletion requires --yes; no command sent")
		}
		return commands.Control(cmd.Context(), opt, "delete", args[0])
	}}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm irreversible deletion of this CRC")
	return cmd
}
