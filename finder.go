package main

import (
	"strings"
	"unicode"
)

type Finder struct {
	findString string
	matches    []position
	current    int
}

type position struct {
	row, col int
}

func (f *Finder) reset() {
	f.matches = nil
	f.findString = ""
}

func (f *Finder) numMatches() int {
	return len(f.matches)
}

func (f *Finder) first() position {
	if f.numMatches() == 0 {
		return position{-1, -1}
	} else {
		return f.matches[0]
	}
}

func (f *Finder) next() position {
	numMatches := f.numMatches()

	if numMatches == 0 {
		return position{-1, -1}
	}

	if numMatches == 1 {
		return f.matches[0]
	}

	if f.current == numMatches-1 {
		f.current = 0
	} else {
		f.current++
	}

	return f.matches[f.current]
}

func (f *Finder) previous() position {
	numMatches := f.numMatches()

	if numMatches == 0 {
		return position{-1, -1}
	}

	if numMatches == 1 {
		return f.matches[0]
	}

	if f.current == 0 {
		f.current = numMatches - 1
	} else {
		f.current--
	}

	return f.matches[f.current]
}

func (f *Finder) find(doc *Document) {
	indices := []position{}
	for i, l := range doc.lines {
		offset := 0
		for {
			idx := strings.Index(l.content[offset:], f.findString)
			if idx == -1 {
				break
			} else {
				absoluteIndex := offset + idx
				indices = append(indices, position{i, absoluteIndex})
				offset = absoluteIndex + len(f.findString)
			}
		}
	}
	f.matches = indices
}

func (f *Finder) editFindString(r rune) {
	switch r {
	case BACKSPACE, DELETE:
		if len(f.findString) > 0 {
			f.findString = f.findString[:len(f.findString)-1]
		}
	default:
		if unicode.IsPrint(r) || r == TAB {
			f.findString += string(r)
		}
	}
}
