package vlog

import (
	"fmt"
	"io"
)

type consoleWriter struct {
	io.WriteCloser
}

func newConsoleWriter() (writer *consoleWriter, err error) {
	newWriter := new(consoleWriter)
	return newWriter, nil
}

func (console *consoleWriter) Write(bytes []byte) (int, error) {
	return fmt.Print(string(bytes))
}

func (console *consoleWriter) Close() error {
	return nil
}

func (console *consoleWriter) String() string {
	return "consoleWriter"
}