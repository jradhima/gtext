package gtext

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
