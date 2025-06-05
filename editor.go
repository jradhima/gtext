package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

const VERSION = "0.0.1"
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
const SHOW_NUMBERS = false
const PAGE_STEP = 20

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
	state     EditorState
	inputChan chan (ReadResult)
	lines     []string
}

type EditorConfig struct {
	numCol       int
	numRow       int
	topMargin    int
	leftMargin   int
	botMargin    int
	showNumbers  bool
	inputTimeout time.Duration
	fileName     string
}

type EditorState struct {
	currentRow int
	currentCol int
}

func NewEditorState(cursorRow int, cursorCol int) EditorState {
	return EditorState{currentRow: cursorRow, currentCol: cursorCol}
}

type ReadResult struct {
	r   rune
	err error
}

func NewEditorConfig(width int, height int, fileName string) EditorConfig {
	topMargin := 1
	botMargin := 1
	leftMargin := 1
	return EditorConfig{
		numCol:       width,
		numRow:       height,
		topMargin:    topMargin,
		leftMargin:   leftMargin,
		botMargin:    botMargin,
		showNumbers:  SHOW_NUMBERS,
		inputTimeout: INPUT_TIMEOUT,
		fileName:     fileName,
	}
}

func NewEditor(r *os.File, config EditorConfig) *Editor {
	initState := NewEditorState(config.topMargin, config.leftMargin)
	return &Editor{
		reader:    bufio.NewReader(r),
		config:    config,
		state:     initState,
		inputChan: make(chan ReadResult),
		lines:     []string{},
	}
}

func (e *Editor) moveCursor(r rune) {
	cr := &e.state.currentRow
	cc := &e.state.currentCol
	switch r {
	case ARROW_UP:
		if *cr > e.config.topMargin {
			*cr--
			if *cc > len(e.lines[*cr-e.config.topMargin]) {
				*cc = len(e.lines[*cr-e.config.topMargin])
			}
		}
	case ARROW_DOWN:
		if *cr < len(e.lines) {
			*cr++
			if *cc > len(e.lines[*cr-e.config.topMargin]) {
				*cc = len(e.lines[*cr-e.config.topMargin])
			}
		}
	case ARROW_LEFT:
		if *cc > e.config.leftMargin {
			*cc--
		} else if *cr > e.config.topMargin {
			*cr--
			*cc = len(e.lines[*cr-e.config.topMargin])
		}
	case ARROW_RIGHT:
		if *cc < len(e.lines[*cr-e.config.topMargin]) {
			*cc++
		} else if *cr < len(e.lines) {
			*cr++
			*cc = e.config.leftMargin
		}
	case PAGE_UP:
		for range PAGE_STEP {
			e.moveCursor(ARROW_UP)
		}
	case PAGE_DOWN:
		for range PAGE_STEP {
			e.moveCursor(ARROW_DOWN)
		}
	case HOME:
		e.state.currentCol = e.config.leftMargin
	case END:
		e.state.currentCol = e.config.numCol
	}
}

func (e *Editor) drawRows(s string) string {
	// welcomeString := fmt.Sprintf("gtext editor -- version %s", VERSION)
	// padding := (e.config.numCol - len(welcomeString)) / 2
	// s += strings.Repeat(" ", padding) + welcomeString + CLEAR_RIGHT + "\r\n"

	for _, line := range e.lines {
		s += line + CLEAR_RIGHT + "\r\n"
	}

	for range e.config.numRow - len(e.lines) - e.config.botMargin - 1 {
		s += "~" + CLEAR_RIGHT + "\r\n"
	}

	helpString := "press Ctrl-Q or z to exit"
	padding := (e.config.numCol - len(helpString)) / 2
	s += fmt.Sprintf("[%d:%d]", e.state.currentRow, e.state.currentCol) + strings.Repeat(" ", padding) + helpString + CLEAR_RIGHT
	return s

}

func (e *Editor) refreshScreen() {
	ab := ""
	ab += HIDE_CURSOR
	ab += TOP_LEFT
	ab = e.drawRows(ab)
	ab += fmt.Sprintf(
		"\x1b[%d;%dH",
		e.state.currentRow,
		e.state.currentCol)
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
	e.loadFile()

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

func (e *Editor) loadFile() {
	file, err := os.OpenFile(e.config.fileName, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		shutdown(fmt.Sprintf("error opening/creating file %s: %s", e.config.fileName, err), 1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		e.lines = append(e.lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		shutdown(fmt.Sprintf("error loading file %s: %s", e.config.fileName, err), 1)
	}

}

func (e *Editor) saveFile() {
	file, err := os.OpenFile(e.config.fileName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		shutdown(fmt.Sprintf("error opening file %s: %s", e.config.fileName, err), 1)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	for _, line := range e.lines {
		_, err := writer.WriteString(fmt.Sprintf("%s\n", line))
		if err != nil {
			shutdown(fmt.Sprintf("error writing line %s: %s", line, err), 1)
		}
	}

}
