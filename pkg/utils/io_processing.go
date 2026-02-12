package utils

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

func PromptBytes(r *bufio.Reader, prompt string) ([]byte, error) {
	for r.Buffered() > 0 {
		_, err := r.Discard(r.Buffered())
		if err != nil {
			return nil, err
		}
	}
	fmt.Println(prompt)
	read, err := r.ReadBytes('\n')
	if errors.Is(err, io.EOF) && (len(read) == 0 || read[len(read)-1] != '\n') {
		return nil, fmt.Errorf("input too large or unexpected EOF: %w", err)
	}
	if err != nil {
		return nil, err
	}

	for r.Buffered() > 0 {
		_, err := r.Discard(r.Buffered())
		if err != nil {
			return nil, err
		}
	}
	return bytes.TrimSpace(read), err
}

func PromptString(r *bufio.Reader, prompt string) (string, error) {
	bytes, err := PromptBytes(r, prompt)
	return string(bytes), err
}
