package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

func Run(fileName string) int {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting raw terminal mode: %v\n", err)
		return 1
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	editor := NewEditor(os.Stdin, fileName)
	shutdownMessage, exitCode := editor.Start()

	fmt.Print(CLEAR + TOP_LEFT + fmt.Sprintf("Exiting: %s\r\n", shutdownMessage))
	time.Sleep(200 * time.Millisecond)
	fmt.Print(CLEAR + TOP_LEFT)

	return exitCode
}

func main() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: gtext [flags] <filename> | [command]\n\n")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  config\tInitializes or prints editor configuration.")
		fmt.Fprintln(os.Stderr, "  help\t\tPrints this help message.")
		fmt.Fprintln(os.Stderr, "  <filename>\tOpens the specified file for editing.")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		flag.PrintDefaults()
	}

	flag.Parse()

	args := flag.Args()

	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	switch args[0] {
	case "config":
		initConfig()
		return
	case "help":
		flag.Usage()
		return
	default:
		if len(args) > 1 {
			fmt.Fprintf(os.Stderr, "Error: Too many arguments: %v\n\n", args[1:])
			flag.Usage()
			os.Exit(1)
		}

		filename := args[0]
		exitCode := Run(filename)
		os.Exit(exitCode)
	}
}
