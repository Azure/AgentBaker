package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/Azure/agentbaker/node-bootstrapper/parser"
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

type App struct {
	// cmdRunner is a function that runs the given command.
	// the goal of this field is to make it easier to test the app by mocking the command runner.
	cmdRunner func(cmd *exec.Cmd) error
}

func cmdRunner(cmd *exec.Cmd) error {
	return cmd.Run()
}

type ProvisionFlags struct {
	ProvisionConfig string
}

func (a *App) Run(ctx context.Context, args []string) int {
	slog.Info("node-bootstrapper started")
	err := a.run(ctx, args)
	exitCode := errToExitCode(err)
	if exitCode == 0 {
		slog.Info("node-bootstrapper finished successfully")
	} else {
		slog.Error("node-bootstrapper failed", "error", err)
	}
	return exitCode
}

func (a *App) run(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("missing command argument")
	}
	switch args[1] {
	case "provision":
		fs := flag.NewFlagSet("provision", flag.ContinueOnError)
		provisionConfig := fs.String("provision-config", "", "path to the provision config file")
		err := fs.Parse(args[2:])
		if err != nil {
			return fmt.Errorf("parse args: %w", err)
		}
		if provisionConfig == nil || *provisionConfig == "" {
			return errors.New("--provision-config is required")
		}
		return a.Provision(ctx, ProvisionFlags{ProvisionConfig: *provisionConfig})
	case "provision-wait":
		fs := flag.NewFlagSet("provision-wait", flag.ContinueOnError)
		timeout := fs.Duration("timeout", 15*time.Minute, "provision wait timeout")
		err := fs.Parse(args[2:])
		if err != nil {
			return fmt.Errorf("parse args: %w", err)
		}
		provisionOutput, err := a.ProvisionWait(ctx, timeout)
		fmt.Println(provisionOutput)
		slog.Info("provision-wait finished", "provisionOutput", provisionOutput)
		return err
	default:
		return fmt.Errorf("unknown command: %s", args[1])
	}
}

func (a *App) Provision(ctx context.Context, flags ProvisionFlags) error {
	inputJSON, err := os.ReadFile(flags.ProvisionConfig)
	if err != nil {
		return fmt.Errorf("open provision file %s: %w", flags.ProvisionConfig, err)
	}

	config := &nbcontractv1.Configuration{}
	err = json.Unmarshal(inputJSON, config)
	if err != nil {
		return fmt.Errorf("unmarshal provision config: %w", err)
	}
	if config.Version != "v0" {
		return fmt.Errorf("unsupported version: %s", config.Version)
	}

	cmd, err := parser.BuildCSECmd(ctx, config)
	if err != nil {
		return fmt.Errorf("build CSE command: %w", err)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	err = a.cmdRunner(cmd)
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	// Is it ok to log a single line? Is it too much?
	slog.Info("CSE finished", "exitCode", exitCode, "stdout", stdoutBuf.String(), "stderr", stderrBuf.String(), "error", err)
	return err
}

// usage example:
// node-bootstrapper provision-wait --timeout=15m
func (a *App) ProvisionWait(ctx context.Context, timeout *time.Duration) (string, error) {
	if _, err := os.Stat(provisionJSONFilePath); err == nil {
		data, err := os.ReadFile(provisionJSONFilePath)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}

	timeoutChan := time.After(*timeout)

	// Set up a channel to listen for interrupts (to cleanly exit)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	// Watch the directory containing the file
	dir := filepath.Dir(provisionJSONFilePath)
	err := os.MkdirAll(dir, 0755) // create the directory if it doesn't exist
	if err != nil {
		return "", err
	}

	// Create inotify instance
	fd, err := unix.InotifyInit()
	if err != nil {
		return "", fmt.Errorf("failed to initialize inotify: %w", err)
	}
	defer unix.Close(fd)

	// Add watch for the file
	wd, err := unix.InotifyAddWatch(fd, dir, unix.IN_CLOSE_WRITE)
	if err != nil {
		return "", fmt.Errorf("failed to add watch for %s: %w", dir, err)
	}
	defer unix.InotifyRmWatch(fd, uint32(wd))

	// Channel to signal when an event is read
	eventChan := make(chan string)
	errorChan := make(chan error)

	// Goroutine to handle reading events
	go func() {

		for {
			// Create a buffer to hold events
			var event [unix.SizeofInotifyEvent + 256]byte // +16 to read filename
			n, err := unix.Read(fd, event[:])             // blocking call
			if err != nil {
				errorChan <- fmt.Errorf("error reading inotify event: %w", err)
				return
			}
			// Process all events in the buffer
			for i := 0; i < n; {
				evt := (*unix.InotifyEvent)(unsafe.Pointer(&event[i]))
				fileName := string(event[unix.SizeofInotifyEvent+i : unix.SizeofInotifyEvent+i+int(evt.Len)])
				fileName = strings.Trim(fileName, "\x00") // remove null byte
				fileName = filepath.Join(dir, fileName)

				// Check for close write event
				if evt.Mask&unix.IN_CLOSE_WRITE != 0 {
					if fileName == provisionJSONFilePath {
						// File has been closed after writing
						data, err := os.ReadFile(provisionJSONFilePath)
						if err != nil {
							errorChan <- fmt.Errorf("error reading file %s: %v", provisionJSONFilePath, err)
							return
						}
						eventChan <- string(data)
					}
				}
				// Move to the next event
				i += unix.SizeofInotifyEvent + int(evt.Len)
			}
		}
	}()

	for {
		select {
		case data := <-eventChan: // Handle data from the goroutine
			return data, nil
		case err := <-errorChan: // Handle errors from the goroutine
			return "", err
		case <-sigChan: // Check for interrupt signal
			// Check for interrupt signal
			return "", fmt.Errorf("terminated by interrupt signal")
		case <-timeoutChan:
			err := a.runSystemctlCommand("status", bootstrapService)
			if err != nil {
				return "", fmt.Errorf("failed to get status of %s: %w", bootstrapService, err)
			}
		default:
		}
	}
}

// runSystemctlCommand is a generic function that runs a systemctl command with specified arguments
func (a *App) runSystemctlCommand(args ...string) error {
	cmd := exec.Command("systemctl", args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	err := a.cmdRunner(cmd)
	return err
}

var _ ExitCoder = &exec.ExitError{}

type ExitCoder interface {
	error
	ExitCode() int
}

func errToExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr ExitCoder
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}
