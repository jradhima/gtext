package main

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func main() {
	// if len(os.Args) != 2 {
	// 	fmt.Println("usage: gtext <filename>")
	// 	os.Exit(1)
	// }

	fmt.Println("Initializing Text Editor...")
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	fileName := "sample.txt" //os.Args[1]

	editor := NewEditor(os.Stdin, fileName)
	editor.Start()
}
