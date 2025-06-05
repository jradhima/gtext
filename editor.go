package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

const VERSION = "0.0.1"
const CTRL_Q rune = '\x11'
const SPACE rune = '\x20'
const CLEAR = "\x1b[2J"
const CLEAR_RIGHT = "\x1b[K"
const TOP_LEFT = "\x1b[H"
const BOTTOM_RIGHT = "\x1b[999C\x1b[999B"
const CURSOR_POSITION = "\x1b[6n"
const HIDE_CURSOR = "\x1b[?25l"
const SHOW_CURSOR = "\x1b[?25h"
const INPUT_TIMEOUT = 100 * time.Millisecond
const SHOW_NUMBERS = true

type Editor struct {
	reader    *bufio.Reader
	config    EditorConfig
	inputChan chan (ReadResult)
}

type EditorConfig struct {
	ncol         int
	nrow         int
	cr           int
	cc           int
	showNumbers  bool
	inputTimeout time.Duration
}

type ReadResult struct {
	r   rune
	err error
}

func NewEditorConfig(width int, height int) EditorConfig {
	return EditorConfig{
		ncol:         width,
		nrow:         height,
		cr:           1,
		cc:           2,
		showNumbers:  SHOW_NUMBERS,
		inputTimeout: INPUT_TIMEOUT,
	}
}

func NewEditor(r *os.File, config EditorConfig) *Editor {
	return &Editor{
		reader:    bufio.NewReader(r),
		config:    config,
		inputChan: make(chan ReadResult)}
}

func (e *Editor) moveCursor(r rune) {
	switch r {
	case 'w':
		if e.config.cr > 1 {
			e.config.cr--
		}
	case 's':
		if e.config.cr < e.config.nrow-1 {
			e.config.cr++
		}
	case 'a':
		if e.config.cc > 2 {
			e.config.cc--
		}
	case 'd':
		if e.config.cc < e.config.ncol-1 {
			e.config.cc++
		}
	}
}

func (e *Editor) drawRows(s string) string {
	for i := range e.config.nrow {
		if e.config.showNumbers {
			s += fmt.Sprintf("%d", i+1)
		} else {
			s += "~"
		}
		if i == e.config.nrow/3 {
			welcomeString := fmt.Sprintf("gtext editor -- version %s", VERSION)
			padding := (e.config.ncol - len(welcomeString)) / 2
			s += strings.Repeat(" ", padding) + welcomeString
		}
		s += CLEAR_RIGHT
		if i != e.config.nrow-1 {
			s += "\r\n"
		}
	}
	return s
}

func (e *Editor) refreshScreen() {
	ab := ""
	ab += HIDE_CURSOR
	ab += TOP_LEFT
	ab = e.drawRows(ab)
	ab += fmt.Sprintf(
		"\x1b[%d;%dH",
		e.config.cr,
		e.config.cc)
	ab += SHOW_CURSOR
	fmt.Print(ab)
}

func (e *Editor) readKeyPresses() {
	for {
		r, _, err := e.reader.ReadRune()
		e.inputChan <- ReadResult{r: r, err: err}
		if err != nil {
			return
		}
	}
}

func (e *Editor) processKeyPress(r rune) {
	switch r {
	case CTRL_Q:
		shutdown("Ctrl+Q", 0)
	case 'z':
		shutdown("z", 0)
	case 'w', 'a', 's', 'd':
		e.moveCursor(r)
	}
}

func (e *Editor) Start() {
	go e.readKeyPresses()

	for {
		select {
		case res := <-e.inputChan:
			if res.err != nil {
				shutdown(fmt.Sprintf("%s", res.err), 1)
			}
			e.processKeyPress(res.r)
		case <-time.After(e.config.inputTimeout):
		}
		e.refreshScreen()
	}
}
