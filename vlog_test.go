package vlog

import (
	"testing"
	//"strconv"
)


func TestTrace(t *testing.T) {
	err := InitLoggerWithFile("vlog.xml")
	if err != nil {
		t.Error(err)
		return
	}
	defer Close()
	
	Trace("Test")
}

/*
func Benchmark_TestTrace(b *testing.B) {
	b.StopTimer()
	err := InitLoggerWithFile("vlog.xml")
	if err != nil {
		b.Error(err)
		return
	}
	defer Close()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		Trace("Debug_", strconv.Itoa(i))	
	}
	
	
}
*/

/*
func TestLogMessages(t *testing.T) {
	lm := logMessage{level:LvTrace, message: "test logMessage"}
	olm := operateLogMessage{level:LvDebug, message: "test operateLogMessage",
		username: "V.Zhang"}
	logMsgs := make(chan messageInterface, 2)
	logMsgs <- lm
	logMsgs <- olm
	
	msg := <- logMsgs
	if m, ok := msg.(logMessage); ok {
		t.Log("msg is a logMessage: ", m.message)
	} else {
		t.Error("msg is not a logMessage")
	}
	
	msg = <- logMsgs
	if m, ok := msg.(operateLogMessage); ok {
		t.Log("msg is a operateLogMessage: ", m.username)
	} else {
		t.Error("msg is not a operateLogMessage")
	}	
}
*/