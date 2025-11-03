package main

import (
	"fmt"
	"strings"
)

const (
	LEFT_MARGIN = 6
)

type View struct {
	rows, cols   int
	rowOffset    int
	topMargin    int
	bottomMargin int
	leftMargin   int
	scrollMargin int
	version      string
}

func NewView(rows, cols int, cfg *Config) *View {
	return &View{
		rows:         rows,
		cols:         cols,
		topMargin:    0,
		bottomMargin: 2,
		leftMargin:   LEFT_MARGIN,
		scrollMargin: cfg.ScrollMargin,
		version:      VERSION,
	}
}

func (v *View) updateSize(rows, cols int) {
	v.rows = rows
	v.cols = cols
}

// --- Rendering entry point ---
func (v *View) Render(mode EditorMode, doc *Document, cfg *Config, cur *Cursor, finder *Finder, cmds *CommandRegistry, bufferLen int, status string) {
	fmt.Print(HIDE_CURSOR + TOP_LEFT)
	fmt.Print(v.drawContent(mode, doc, cfg, cur, finder, cmds, bufferLen, status))
	row, col := cur.screenCoords()
	fmt.Printf("\x1b[%d;%dH%s", row, col, SHOW_CURSOR)
}

// --- Draw all visible lines and footer ---
func (v *View) drawContent(mode EditorMode, doc *Document, cfg *Config, cur *Cursor, finder *Finder, cmds *CommandRegistry, bufferLen int, status string) string {
	var builder strings.Builder
	visibleRows := v.rows - v.bottomMargin

	for screenRow := 0; screenRow < visibleRows; screenRow++ {
		docRow := v.rowOffset + screenRow
		lineText := v.renderLine(doc, docRow, cfg)
		builder.WriteString(lineText)
		builder.WriteString(CLEAR_RIGHT + "\r\n")
	}
	builder.WriteString(v.makeFooter(mode, doc, cfg, cur, finder, cmds, bufferLen, status))
	return builder.String()
}

// --- Draw a single line ---
func (v *View) renderLine(doc *Document, row int, cfg *Config) string {
	sideWidth := v.leftMargin - 1
	if row >= doc.lineCount() {
		return fmt.Sprintf("%s~", strings.Repeat(" ", sideWidth-1))
	}
	lineNum := fmt.Sprintf("%d", row+1)
	if !cfg.ShowLineNumbers {
		lineNum = "~"
	}
	padding := strings.Repeat(" ", sideWidth-len(lineNum))
	return padding + lineNum + " " + doc.lines[row].render
}

func (v *View) makeFooter(mode EditorMode, doc *Document, cfg *Config, cur *Cursor, finder *Finder, cmds *CommandRegistry, bufferLen int, status string) string {
	var builder strings.Builder
	builder.WriteString(BLACK_ON_WHITE)

	switch mode {
	case EditMode:
		builder.WriteString(buildCommandHintLine(cmds))
	case FindMode:
		builder.WriteString("Ctrl-F: Exit find mode | Enter: Search substring | Next: →↓ | Prev: ←↑ | ")
		builder.WriteString(fmt.Sprintf("[searching for: %s_]", finder.findString))
		if finder.numMatches() > 0 {
			builder.WriteString(fmt.Sprintf(" [match: %d/%d]", finder.current+1, finder.numMatches()))
		}
	}
	builder.WriteString(CLEAR_RIGHT + RESET + "\r\n")

	row, col := cur.coords()
	dirtyMarker := ""
	if doc.dirty {
		dirtyMarker = "*"
	}

	editorState := fmt.Sprintf("[%d:%d] [lines: %d] [file: %s%s]", row+1, col+1, doc.lineCount(), doc.fileName, dirtyMarker)
	if bufferLen > 0 {
		editorState += fmt.Sprintf(" [buffer: %d lines]", bufferLen)
	}
	center := fmt.Sprintf("gtext v%s", v.version)

	// compute padding
	leftPadding := max((v.cols-len(center))/2-len(editorState), 0)
	rightPadding := max((v.cols-len(center))/2-len(status), 0)

	builder.WriteString(editorState)
	builder.WriteString(strings.Repeat(" ", leftPadding))
	builder.WriteString(center)
	builder.WriteString(strings.Repeat(" ", rightPadding))
	builder.WriteString(status)
	builder.WriteString(CLEAR_RIGHT)

	return builder.String()
}

// --- Scroll logic ---
func (v *View) updateScroll(cursorRow, totalLines int) {
	for {
		screenY := cursorRow - v.rowOffset
		if screenY < v.topMargin+v.scrollMargin {
			if v.rowOffset > 0 {
				v.rowOffset--
			} else {
				break
			}
		} else if screenY >= v.rows-v.bottomMargin-v.scrollMargin {
			maxOffset := (totalLines - 1) - (v.topMargin + v.scrollMargin)
			if v.rowOffset < maxOffset {
				v.rowOffset++
			} else {
				break
			}
		} else {
			break
		}
	}
}

func buildCommandHintLine(cr *CommandRegistry) string {
	var parts []string
	for _, key := range cr.order {
		cmd := cr.cmds[key]
		parts = append(parts, fmt.Sprintf("%s: %s", cmd.name, cmd.desc))
	}
	return strings.Join(parts, " | ")
}
