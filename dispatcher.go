package vlog

import (
	"errors"
)

type dispatcher struct {
	writers     []*formattedWriter
}

func createDispatcher(receivers []*formattedWriter) (*dispatcher, error) {
	if receivers == nil || len(receivers) == 0 {
		return nil, errors.New("dispatcher creation error: receivers can not be empty.")
	}
	disp := new(dispatcher)
	disp.writers = receivers
	return disp, nil
}

func (disp *dispatcher) Dispatch(message string, level LogLevel, 
	context runtimeContextInterface, errorFunc func(err error)) {
	
	for _, writer := range disp.writers {
		err := writer.Write(message, level, context)
		if err != nil {
			errorFunc(err)
		}
	}
}

func (disp *dispatcher) Close() error {
	errMsg := ""
	for _, fmtWriter := range disp.writers {
		err := fmtWriter.Close()
		if err != nil {
			errMsg += err.Error() + ","
			continue
		}
	}
	
	if errMsg != "" {
		return errors.New("some writer closed error: " + errMsg[:len(errMsg) - 1])
	}
	return nil
}