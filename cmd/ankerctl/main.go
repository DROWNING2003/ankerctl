package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/commands"
	"github.com/spf13/cobra"
)

var opt = commands.Options{Timeout: 2 * time.Second, Out: os.Stdout, Err: os.Stderr}

func main() {
	root := &cobra.Command{Use: "ankerctl", Short: "Anker A210C command-line controller", Version: buildVersion()}
	root.PersistentFlags().StringVar(&opt.Port, "port", "", "serial port")
	root.PersistentFlags().BoolVar(&opt.JSON, "json", false, "JSON output")
	root.PersistentFlags().DurationVar(&opt.Timeout, "timeout", 2*time.Second, "communication timeout")
	root.PersistentFlags().BoolVar(&opt.Debug, "debug", false, "protocol debug")
	root.AddCommand(&cobra.Command{Use: "devices", RunE: func(*cobra.Command, []string) error { return commands.Devices(opt) }})
	root.AddCommand(&cobra.Command{Use: "info", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return commands.Info(opt) }}, unsupported("status"), &cobra.Command{Use: "ports", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return commands.Ports(opt) }}, unsupported("monitor"), unsupported("doctor"))
	display := &cobra.Command{Use: "display"}
	display.AddCommand(&cobra.Command{Use: "get", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return commands.DisplayGet(opt) }})
	b := &cobra.Command{Use: "brightness <0-100>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		v, e := strconv.Atoi(args[0])
		if e != nil || v < 10 || v > 100 {
			return fmt.Errorf("brightness must be between 10 and 100")
		}
		return commands.Brightness(opt, v)
	}}
	display.AddCommand(b, unsupported("timeout"))
	root.AddCommand(display)
	screen := &cobra.Command{Use: "screen"}
	screen.AddCommand(&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return commands.ScreenGet(opt) }}, &cobra.Command{Use: "upload <file>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error { return commands.UploadScreen(opt, args[0]) }}, unsupported("delete"))
	root.AddCommand(screen, timerCommand())
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
func unsupported(use string) *cobra.Command {
	return &cobra.Command{Use: use, RunE: func(*cobra.Command, []string) error { return commands.Unsupported(opt, use) }}
}
