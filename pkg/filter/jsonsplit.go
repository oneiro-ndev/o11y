package filter

import (
	"bytes"
	"errors"
)

// JSONSplit is compatible with bufio.SplitFunc; it reads a single JSON object from the
// input stream. It must be a JSON object delimited by {} -- it doesn't accept just any json.
// It discards any non-objects between objects.
// We did not use a json.Decoder because that system documents that it might overrun buffers.
func JSONSplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	start := bytes.IndexByte(data, '{')
	if start == -1 {
		return 0, nil, nil
	}
	end, ok := matchBrace(data, start+1)
	if !ok {
		if atEOF {
			return 0, nil, errors.New("incomplete JSON object")
		}
		return 0, nil, nil
	}
	end++
	return end, data[start:end], nil
}

func matchBrace(data []byte, start int) (int, bool) {
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '}':
			return i, true
		case '"':
			newi, ok := matchQuote(data, i+1)
			if !ok {
				return -1, false
			}
			i = newi
		case '{':
			newi, ok := matchBrace(data, i+1)
			if !ok {
				return -1, false
			}
			i = newi
		}
	}
	return -1, false
}

func matchQuote(data []byte, start int) (int, bool) {
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++ // skip an extra char
		case '"':
			return i, true
		}
	}
	return -1, false
}
