package main

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

func shutdown(s string, code int) {
	fmt.Print(CLEAR)
	fmt.Print(TOP_LEFT)

	fmt.Printf("Exiting: %s\r\n", s)
	time.Sleep(250 * time.Millisecond)

	fmt.Print(CLEAR)
	fmt.Print(TOP_LEFT)
	os.Exit(code)
}

func getWindowSize() (int, int) {
	ncol, nrow, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || (ncol == 0 && nrow == 0) {
		return getWindowSizeFallback()
	}
	return ncol, nrow
}

func getWindowSizeFallback() (int, int) {
	size, err := fmt.Print(BOTTOM_RIGHT)
	if err != nil {
		shutdown(fmt.Sprintf("%s", err), 1)
	} else if size != 12 {
		shutdown("Window size escape sequence error", 1)
	}
	return getCursorPosition()
}

func getCursorPosition() (int, int) {
	size, err := fmt.Print(CURSOR_POSITION)
	if err != nil {
		shutdown(fmt.Sprintf("%s", err), 1)
	} else if size != 4 {
		shutdown("cursor position escape sequence error", 1)
	}

	reader := bufio.NewReader(os.Stdin)
	b, err := reader.ReadBytes('R')
	if err != nil {
		shutdown(fmt.Sprintf("%s", err), 1)
	} else if b[0] != '\x1b' || b[1] != '[' {
		shutdown("cursor position return not valid", 1)
	}
	var nrow, ncol int
	_, err = fmt.Sscanf(fmt.Sprintf("%s", b[1:]), "[%d;%dR", &nrow, &ncol)
	if err != nil {
		shutdown(fmt.Sprintf("error parsing: %s", err), 1)
	}
	return nrow, ncol
}

func main() {
	if len(os.Args) != 2 {
		shutdown("usage: gtext <filename>", 1)
	}

	fmt.Println("Initializing Text Editor...")
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	ncol, nrow := getWindowSize()

	fileName := os.Args[1]

	editorConfig := NewEditorConfig(ncol, nrow, fileName)
	editor := NewEditor(os.Stdin, editorConfig)
	editor.Start()
}
