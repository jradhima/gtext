package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

const VERSION = "0.0.1"
const LEFT_MARGIN = 3
const TOP_MARGIN = 1
const ESCAPE rune = '\x1b'
const CSI byte = '['
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

const (
	ARROW_UP    rune = 0xE000
	ARROW_DOWN  rune = 0xE001
	ARROW_RIGHT rune = 0xE002
	ARROW_LEFT  rune = 0xE003
	PAGE_UP     rune = 0xE004
	PAGE_DOWN   rune = 0xE005
	HOME        rune = 0xE006
	END         rune = 0xE007
)

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
		cr:           TOP_MARGIN,
		cc:           LEFT_MARGIN,
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
	case ARROW_UP:
		if e.config.cr > TOP_MARGIN {
			e.config.cr--
		}
	case ARROW_DOWN:
		if e.config.cr < e.config.nrow {
			e.config.cr++
		}
	case ARROW_LEFT:
		if e.config.cc > LEFT_MARGIN {
			e.config.cc--
		} else if e.config.cr > TOP_MARGIN {
			e.config.cr--
			e.config.cc = e.config.ncol
		}
	case ARROW_RIGHT:
		if e.config.cc < e.config.ncol {
			e.config.cc++
		} else if e.config.cr < e.config.nrow {
			e.config.cr++
			e.config.cc = LEFT_MARGIN
		}
	case PAGE_UP:
		e.config.cr = TOP_MARGIN
	case PAGE_DOWN:
		e.config.cr = e.config.nrow
	case HOME:
		e.config.cc = LEFT_MARGIN
	case END:
		e.config.cc = e.config.ncol
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
		if err != nil {
			e.inputChan <- ReadResult{r: ESCAPE, err: err}
		}
		if r == ESCAPE {
			b, err := e.reader.Peek(1)
			if err != nil {
				e.inputChan <- ReadResult{r: ESCAPE, err: err}
			}
			if len(b) == 1 && b[0] == CSI {
				_, _, err := e.reader.ReadRune()
				if err != nil {
					e.inputChan <- ReadResult{r: ESCAPE, err: err}
				}
				b1, _, err := e.reader.ReadRune()
				if err != nil {
					e.inputChan <- ReadResult{r: ESCAPE, err: err}
				}
				switch b1 {
				case 'A':
					e.inputChan <- ReadResult{r: ARROW_UP, err: err}
				case 'B':
					e.inputChan <- ReadResult{r: ARROW_DOWN, err: err}
				case 'C':
					e.inputChan <- ReadResult{r: ARROW_RIGHT, err: err}
				case 'D':
					e.inputChan <- ReadResult{r: ARROW_LEFT, err: err}
				case '5':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: PAGE_UP, err: err}
					}
				case '6':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: PAGE_DOWN, err: err}
					}
				case '1', '7', 'H':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: HOME, err: err}
					}
				case '4', '8', 'F':
					b2, _, err := e.reader.ReadRune()
					if err != nil {
						e.inputChan <- ReadResult{r: ESCAPE, err: err}
					}
					if b2 == '~' {
						e.inputChan <- ReadResult{r: END, err: err}
					}

				}

			}
		}
		e.inputChan <- ReadResult{r: r, err: err}
	}
}

func (e *Editor) processKeyPress(r rune) {
	switch r {
	case CTRL_Q:
		shutdown("Ctrl+Q", 0)
	case 'z':
		shutdown("z", 0)
	case ARROW_UP, ARROW_DOWN, ARROW_RIGHT, ARROW_LEFT, PAGE_UP, PAGE_DOWN, HOME, END:
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
