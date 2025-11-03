package main

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

// const and setup

const (
	// ASCII control characters
	CTRL_F rune = 0x06
	CTRL_Q rune = 0x11
	CTRL_S rune = 0x13
	CTRL_V rune = 0x16
	CTRL_C rune = 0x03
	CTRL_X rune = 0x18

	// Common keyboard characters
	BACKSPACE rune = 0x08
	TAB       rune = 0x09
	RETURN    rune = 0x0d
	ESCAPE    rune = 0x1b
	SPACE     rune = 0x20
	CSI       rune = 0x5b
	DELETE    rune = 0x7f
)

const (
	ARROW_UP    rune = 0xE000
	ARROW_DOWN  rune = 0xE001
	ARROW_RIGHT rune = 0xE002
	ARROW_LEFT  rune = 0xE003
	PAGE_UP     rune = 0xE004
	PAGE_DOWN   rune = 0xE005
	HOME        rune = 0xE006
	END         rune = 0xE007
	NEW_LINE    rune = 0xE008
)

const (
	CLEAR           = "\x1b[2J"            // Clear screen
	CLEAR_RIGHT     = "\x1b[K"             // Clear from cursor to end of line
	TOP_LEFT        = "\x1b[H"             // Move cursor to top-left corner
	BOTTOM_RIGHT    = "\x1b[999C\x1b[999B" // Move cursor to bottom-right (approximated)
	CURSOR_POSITION = "\x1b[6n"            // Request cursor position report
	HIDE_CURSOR     = "\x1b[?25l"          // Hide cursor
	SHOW_CURSOR     = "\x1b[?25h"          // Show cursor
)

const (
	HIGHLIGHT_MATCH = "\x1b[30;43m"
	BLACK_ON_WHITE  = "\x1b[30;47m"       // Set foreground to black, background to white
	BLACK_ON_GREY   = "\x1b[30;48;5;240m" // Set foreground to black, background to grey
	RESET           = "\x1b[0m"           // Reset all SGR (Select Graphic Rendition) parameters
)

const (
	INPUT_TIMEOUT = 100 * time.Millisecond // Timeout for input operations
	PAGE_STEP     = 20                     // Number of lines to scroll for page up/down
	EXPAND_TABS   = false                  // Whether to expand tabs to spaces
	TAB_SIZE      = 4                      // Number of spaces for a tab if expanded
)

const ErrWindowNullSize = gtextError("window size reported as 0,0")

func getWindowSize() (int, int, error) {
	ncol, nrow, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0, 0, fmt.Errorf("could not get window size: %w", err)
	}
	if ncol == 0 && nrow == 0 {
		return 0, 0, ErrWindowNullSize
	}
	return nrow, ncol, nil
}

func ReadKey(r *bufio.Reader) (rune, error) {
	ch, _, err := r.ReadRune()
	if err != nil {
		return 0, err
	}
	if ch != ESCAPE {
		return ch, nil
	}
	seq, err := r.Peek(2)
	if len(seq) != 2 || seq[0] != byte(CSI) {
		return ESCAPE, nil
	}
	if err != nil {
		return 0, err
	}

	r.ReadRune()
	ch, _, _ = r.ReadRune()
	switch ch {
	case 'A':
		return ARROW_UP, nil
	case 'B':
		return ARROW_DOWN, nil
	case 'C':
		return ARROW_RIGHT, nil
	case 'D':
		return ARROW_LEFT, nil
	case 'H':
		return HOME, nil
	case 'F':
		return END, nil
	case '5':
		ch, _, _ := r.ReadRune()
		if ch == '~' {
			return PAGE_UP, nil
		}
	case '6':
		ch, _, _ := r.ReadRune()
		if ch == '~' {
			return PAGE_DOWN, nil
		}
	case '1', '7':
		ch, _, _ := r.ReadRune()
		if ch == '~' {
			return HOME, nil
		}
	case '4', '8':
		ch, _, _ := r.ReadRune()
		if ch == '~' {
			return END, nil
		}
	default:
		return ESCAPE, nil
	}
	return 0, ErrReturnSeqTerminator
}
