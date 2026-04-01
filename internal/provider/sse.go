package provider

import (
	"bufio"
	"io"
	"strings"
)

type SSEEvent struct {
	Event string
	Data  string
}

type SSEParser struct {
	scanner *bufio.Scanner
}

func NewSSEParser(r io.Reader) *SSEParser {
	return &SSEParser{scanner: bufio.NewScanner(r)}
}

func (p *SSEParser) Next() (SSEEvent, error) {
	var event SSEEvent
	var dataLines []string
	hasData := false

	for p.scanner.Scan() {
		line := p.scanner.Text()

		if line == "" {
			if hasData || event.Event != "" {
				event.Data = strings.Join(dataLines, "\n")
				return event, nil
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			continue
		}

		field, value := parseSSEField(line)

		switch field {
		case "data":
			hasData = true
			dataLines = append(dataLines, value)
		case "event":
			event.Event = value
		case "id", "retry":
			// ignored per spec
		}
	}

	if err := p.scanner.Err(); err != nil {
		return SSEEvent{}, err
	}

	if hasData || event.Event != "" {
		event.Data = strings.Join(dataLines, "\n")
		return event, nil
	}

	return SSEEvent{}, io.EOF
}

func parseSSEField(line string) (field, value string) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return line, ""
	}
	field = line[:idx]
	value = line[idx+1:]
	value = strings.TrimPrefix(value, " ")
	return field, value
}
