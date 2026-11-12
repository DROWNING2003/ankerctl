package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/DROWNING2003/ankerctl/internal/commands"
	"github.com/DROWNING2003/ankerctl/internal/protocol"
	"github.com/spf13/cobra"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run owns all error rendering, including parse/validation errors. Cobra never
// mixes a usage banner or plain-text errors into machine-readable output.
func run(ctx context.Context, args []string, out, stderr io.Writer) int {
	root := newRoot(out, stderr)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return 0
	}
	asJSON, _ := root.PersistentFlags().GetBool("json")
	// Unknown-command/flag errors can occur before Cobra parses persistent flags.
	if !asJSON {
		for _, arg := range args {
			if arg == "--" {
				break
			}
			if arg == "--json" || arg == "--json=true" {
				asJSON = true
			}
			if arg == "--json=false" {
				asJSON = false
			}
		}
	}
	code := "ERROR"
	exit := 1
	switch {
	case errors.Is(err, context.Canceled):
		code = "CANCELED"
		exit = 130
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, protocol.ErrTimeout):
		code = "TIMEOUT"
	case errors.Is(err, protocol.ErrUnsupported):
		code = "UNSUPPORTED"
	}
	if asJSON {
		_ = json.NewEncoder(out).Encode(map[string]any{"error": map[string]string{"code": code, "message": err.Error()}})
	} else {
		fmt.Fprintln(stderr, "Error:", err)
	}
	return exit
}
func newRoot(out, stderr io.Writer) *cobra.Command {
	opt := &commands.Options{Timeout: 2 * time.Second, Out: out, Err: stderr}
	root := &cobra.Command{Use: "ankerctl", Short: "Anker A210C command-line controller", Version: buildVersion(), SilenceErrors: true, SilenceUsage: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }, PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		opt.Context = cmd.Context()
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		if opt.Timeout <= 0 {
			return fmt.Errorf("timeout must be positive")
		}
		return nil
	}}
	root.SetOut(out)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&opt.Port, "port", "", "serial port")
	root.PersistentFlags().BoolVar(&opt.JSON, "json", false, "JSON output")
	root.PersistentFlags().DurationVar(&opt.Timeout, "timeout", 2*time.Second, "communication timeout")
	root.PersistentFlags().BoolVar(&opt.Debug, "debug", false, "protocol debug")
	root.AddCommand(&cobra.Command{Use: "devices", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return commands.Devices(*opt) }})
	root.AddCommand(&cobra.Command{Use: "info", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return commands.Info(*opt) }}, unsupported("status", opt), &cobra.Command{Use: "ports", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return commands.Ports(*opt) }}, unsupported("monitor", opt), unsupported("doctor", opt))
	display := &cobra.Command{Use: "display"}
	display.AddCommand(&cobra.Command{Use: "get", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return commands.DisplayGet(*opt) }})
	brightness := &cobra.Command{Use: "brightness <10-100>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error {
		v, err := strconv.Atoi(args[0])
		if err != nil || v < 10 || v > 100 {
			return fmt.Errorf("brightness must be between 10 and 100")
		}
		return commands.Brightness(*opt, v)
	}}
	display.AddCommand(brightness, controlCommand("timeout <duration>", "timeout", cobra.ExactArgs(1), opt), controlCommand("language <en|zh|ja|de>", "language", cobra.ExactArgs(1), opt))
	root.AddCommand(display)
	screen := &cobra.Command{Use: "screen"}
	screen.AddCommand(&cobra.Command{Use: "list", Short: "Show current wallpaper and slot occupancy (not a full image list)", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return commands.ScreenGet(*opt) }}, &cobra.Command{Use: "upload <file>", Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error { return commands.UploadScreen(*opt, args[0]) }}, deleteScreenCommand(opt), controlCommand("on", "screen-on", cobra.NoArgs, opt), controlCommand("off", "screen-off", cobra.NoArgs, opt), controlCommand("select <crc>", "select", cobra.ExactArgs(1), opt))
	screen.AddCommand(carouselCommand(opt))
	root.AddCommand(screen, timerCommand(opt), reminderCommand(opt), focusCommand(opt))
	configureCommandGroups(root)
	return root
}

// Cobra otherwise prints help and exits successfully for unknown arguments on
// non-runnable grouping commands. Make every group validate positional input.
func configureCommandGroups(cmd *cobra.Command) {
	if cmd.HasSubCommands() && cmd.Run == nil && cmd.RunE == nil {
		cmd.Args = cobra.NoArgs
		cmd.RunE = func(c *cobra.Command, _ []string) error { return c.Help() }
	}
	for _, child := range cmd.Commands() {
		configureCommandGroups(child)
	}
}
func unsupported(use string, opt *commands.Options) *cobra.Command {
	return &cobra.Command{Use: use, Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return commands.Unsupported(*opt, use) }}
}
