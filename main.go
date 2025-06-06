package main

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

func runEditor() int {
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	editor := NewEditor(os.Stdin, os.Args[1])
	shutdownMessage, exitCode := editor.Start()

	fmt.Print(CLEAR + TOP_LEFT + fmt.Sprintf("Exiting: %s\r\n", shutdownMessage))
	time.Sleep(200 * time.Millisecond)
	fmt.Print(CLEAR + TOP_LEFT)

	return exitCode
}

func main() {
	args := os.Args

	if len(args) == 2 {
		switch args[1] {
		case "config":
			InitConfig()
			return
		case "help":
			fmt.Println("usage: gtext <filename> or gtext init")
			return
		default:
			exitCode := runEditor()
			os.Exit(exitCode)
		}
	} else {
		fmt.Println("usage: gtext <filename> or gtext init")
		os.Exit(1)
	}

}
