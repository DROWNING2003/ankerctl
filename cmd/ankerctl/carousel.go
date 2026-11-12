package main

import (
	"github.com/DROWNING2003/ankerctl/internal/commands"
	"github.com/spf13/cobra"
)

// carouselCommand is attached under screen by the root command's owner.
func carouselCommand() *cobra.Command {
	root := &cobra.Command{Use: "carousel", Short: "Control the existing wallpaper carousel without clearing stored images (experimental)"}
	for _, operation := range []string{"status", "on", "off"} {
		root.AddCommand(&cobra.Command{Use: operation, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			return commands.Carousel(cmd.Context(), opt, operation, "")
		}})
	}
	root.AddCommand(&cobra.Command{Use: "interval <seconds|duration>", Short: "Set interval to 3..3600 whole seconds (e.g. 60 or 1m)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Carousel(cmd.Context(), opt, "interval", args[0])
	}})
	return root
}
