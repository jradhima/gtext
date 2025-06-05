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
	if len(os.Args) != 2 {
		fmt.Println("usage: gtext <filename>")
		os.Exit(1)
	}

	exitCode := runEditor()
	os.Exit(exitCode)
}
