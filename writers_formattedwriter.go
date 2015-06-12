package vlog

import (
	"errors"
	"fmt"
	"io"
)

type formattedWriter struct {
	io.WriteCloser
	writer           io.WriteCloser
	formatter        *formatter //消息格式化器
	allowedLevelList map[LogLevel]bool
}

func newFormattedWriter(writer io.WriteCloser, formatter *formatter,
	allowedLevelList map[LogLevel]bool) (fmtWriter *formattedWriter, err error) {
	
	if formatter == nil {
		return nil, errors.New("formatter can not be nil")
	}
	fmtWriter = new(formattedWriter)
	fmtWriter.writer = writer
	fmtWriter.formatter = formatter
	fmtWriter.allowedLevelList = allowedLevelList
	return fmtWriter, nil
}

func (formattedWriter *formattedWriter) Write(message string, level LogLevel, context runtimeContextInterface)(err error) {
	defer func() {
		if e, ok := recover().(error); ok {
			err = e
			writeRuntimeError(e)
		}
	} ()
	isAllowed, ok := formattedWriter.allowedLevelList[level]
	if isAllowed && ok {
		str := formattedWriter.formatter.Format(message, level, context)
		writer := formattedWriter.writer
		w, ok := writer.(*ruleFileWriter)
		if ok {
			w.formatFileName(level, context)
		}
		_, err = writer.Write([]byte(str))
	}
	return err
}

func (fmtWriter *formattedWriter) Close() error {
	if fmtWriter.writer != nil {
		err := fmtWriter.writer.Close()
		fmtWriter.writer = nil
		return err
	}
	return nil
}

func (fmtWriter *formattedWriter) String() string {
	return "formattedWriter: formatter=[" + fmt.Sprint(fmtWriter.formatter, "]") +
		", writer=[" + fmt.Sprint(fmtWriter.writer, "]")
}