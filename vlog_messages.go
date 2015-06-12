package vlog

import (
	"fmt"
)

type logMessage struct {
	level   LogLevel
	message string
	context runtimeContextInterface
}

func newLogMessage(level LogLevel, params []interface{}) {
	context, err := specificContext(2)
	if err != nil {
		errorFunc(err)
		return
	}
	message := logMessage{}
	message.level = level
	message.message = fmt.Sprint(params...)
	message.context = context
	pushLogMessageToChannel(message)
}

func newFormatLogMessage(level LogLevel, fmtString string, params []interface{}) {
	context, err := specificContext(2)
	if err != nil {
		errorFunc(err)
		return
	}
	message := logMessage{}
	message.level = level
	message.message = fmt.Sprintf(fmtString, params...)
	message.context = context
	pushLogMessageToChannel(message)
}