package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"
)

// const and setup

const VERSION = "0.0.1"
const ESCAPE rune = '\x1b'
const CSI byte = '['
const CTRL_Q rune = '\x11'
const SPACE rune = '\x20'
const BACKSPACE rune = '\x08'
const DELETE rune = 127
const RETURN rune = 13
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

// structs and types

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
	row int
	col int
}

type ReadResult struct {
	r   rune
	err error
}

func (e *Editor) getSliceCoords() (int, int) {
	return e.state.row - e.config.topMargin, e.state.col - e.config.leftMargin
}

func NewEditorState(row int, col int) EditorState {
	return EditorState{row: row, col: col}
}

func NewEditorConfig(width int, height int, fileName string) EditorConfig {
	topMargin := 1
	leftMargin := 1
	botMargin := 1

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

// terminal functionality

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

func (e *Editor) moveCursor(r rune) {
	switch r {
	case ARROW_UP:
		if e.state.row > e.config.topMargin {
			e.state.row--
			lineRow, _ := e.getSliceCoords()
			if e.state.col > len(e.lines[lineRow]) {
				e.state.col = len(e.lines[lineRow]) + 1
			}
		}
	case ARROW_DOWN:
		if e.state.row < len(e.lines) {
			e.state.row++
			lineRow, _ := e.getSliceCoords()
			if e.state.col > len(e.lines[lineRow]) {
				e.state.col = len(e.lines[lineRow]) + 1
			}
		}
	case ARROW_LEFT:
		if e.state.col > e.config.leftMargin {
			e.state.col--
		} else if e.state.row > e.config.topMargin {
			e.state.row--
			lineRow, _ := e.getSliceCoords()
			e.state.col = len(e.lines[lineRow]) + 1
		}
	case ARROW_RIGHT:
		lineRow, _ := e.getSliceCoords()
		if e.state.col <= len(e.lines[lineRow]) {
			e.state.col++
		} else if e.state.row < len(e.lines) {
			e.state.row++
			e.state.col = e.config.leftMargin
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
		e.state.col = e.config.leftMargin
	case END:
		e.state.col = e.config.numCol
	}
}

// rendering

func (e *Editor) makeFooter() string {
	helpString := "exit: Ctrl-Q or z"
	welcomeString := fmt.Sprintf("gtext editor -- version %s", VERSION)
	editorState := fmt.Sprintf("[%d:%d] lines: %d", e.state.row, e.state.col, len(e.lines))

	leftPadding := (e.config.numCol-len(welcomeString))/2 - len(editorState)
	rightPadding := (e.config.numCol-len(welcomeString))/2 - len(helpString)

	s := editorState + strings.Repeat(" ", leftPadding) + welcomeString + strings.Repeat(" ", rightPadding) + helpString + CLEAR_RIGHT
	return s
}

func (e *Editor) drawRows(s string) string {
	for _, line := range e.lines {
		s += line + CLEAR_RIGHT + "\r\n"
	}

	for range e.config.numRow - len(e.lines) - e.config.botMargin {
		s += "~" + CLEAR_RIGHT + "\r\n"
	}

	s += e.makeFooter()

	return s

}

func (e *Editor) refreshScreen() {
	ab := ""
	ab += HIDE_CURSOR
	ab += TOP_LEFT
	ab = e.drawRows(ab)
	ab += fmt.Sprintf(
		"\x1b[%d;%dH",
		e.state.row,
		e.state.col)
	ab += SHOW_CURSOR
	fmt.Print(ab)
}

// text editing

func (e *Editor) processKeyPress(r rune) {
	switch r {
	case CTRL_Q:
		shutdown("Ctrl+Q", 0)
	case 'z':
		shutdown("z", 0)
	case ARROW_UP, ARROW_DOWN, ARROW_RIGHT, ARROW_LEFT, PAGE_UP, PAGE_DOWN, HOME, END:
		e.moveCursor(r)
	case BACKSPACE, DELETE:
		e.backspace()
	case RETURN:
		e.newLine()
	default:
		if unicode.IsPrint(r) {
			e.write(r)
		}
	}
}

func (e *Editor) write(r rune) {
	row, idx := e.getSliceCoords()
	line := e.lines[row]
	if line == "" {
		e.lines[row] = fmt.Sprintf("%c", r)
	} else if idx == 0 {
		e.lines[row] = fmt.Sprintf("%c%s", r, line)
	} else if idx == len(line)+1 {
		e.lines[row] = fmt.Sprintf("%s%c", line, r)
	} else {
		e.lines[row] = fmt.Sprintf("%s%c%s", line[:idx], r, line[idx:])
	}
	e.moveCursor(ARROW_RIGHT)
}

func (e *Editor) backspace() {
	row, idx := e.getSliceCoords()
	line := e.lines[row]
	if len(e.lines) == 0 { // case empty file
		return
	} else if idx == 0 { //case first position
		if row == 0 { // top row do nothing
			return
		} else if row == len(e.lines)-1 { // last row simple operation
			e.lines[row-1] = fmt.Sprintf("%s%s", e.lines[row-1], line)
			e.lines = e.lines[:row]
		} else { // in the middle need appending
			e.lines[row-1] = fmt.Sprintf("%s%s", e.lines[row-1], line)
			e.lines = append(e.lines[:row], e.lines[row+1:]...)
		}
	} else { // somewhere in the middle
		e.lines[row] = fmt.Sprintf("%s%s", line[:idx-1], line[idx:])
	}
	e.moveCursor(ARROW_LEFT)
}

func (e *Editor) newLine() {
	row, _ := e.getSliceCoords()
	newLines := append(e.lines[:row], "")
	newLines = append(newLines, e.lines[row:]...)
	e.lines = newLines
	// currentIndex := e.state.col - 1
	// line := e.lines[currentLine]
	// if line == "" {

	// }
	// e.lines[currentLine] = fmt.Sprintf("%s%c%s", line[:currentIndex], r, line[currentIndex:])
	// e.moveCursor(ARROW_RIGHT)
}

// higher level functionality
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
