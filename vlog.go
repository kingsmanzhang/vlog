//
// 2014-04-11 by V.Zhang
//
package vlog

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var RUNTIME_ERROR_LOG_FILENAME = "vlog_runtime_error.log"

var vloggerInstance *logger
var logMessages chan logMessage
//const logSepStr = "|"

type logger struct {
	lock     sync.Mutex
	maxLevel LogLevel
	minLevel LogLevel
	disp     *dispatcher
	isClosed bool
}

func getLoggerInstance(config *configuration) (log *logger, err error) {
	
	disp, err := createDispatcher(config.writers)
	if err != nil {
		return nil, err
	}
	log = new(logger)
	log.maxLevel = config.maxLevel
	log.minLevel = config.minLevel
	log.disp = disp
	log.isClosed = false	
	return log, nil
}

func (log *logger) start() {
	go dispatchLogMessage()
}

func pushLogMessageToChannel(lm logMessage) {
	if vloggerInstance != nil {
		if lm.level >= vloggerInstance.minLevel && lm.level <= vloggerInstance.maxLevel {
			logMessages <- lm
		}
	} else {
		panicLoggerInstanceNotBeCreated()
	}
}

func dispatch(lm logMessage) {
	vloggerInstance.disp.Dispatch(lm.message, lm.level, lm.context, errorFunc)
}

func dispatchLogMessage() {
	//另一种方式
	//for {
	//	fmt.Println("等待日志消息....")
	//	select {
	//		case lm := <-logMessages:
	//			fmt.Println("发送日志消息....")
	//			vloggerInstance.disp.Dispatch(lm.message, lm.level, lm.context, errorFunc)
	//		case isClosed := <- vloggerInstance.isClosed:
	//			fmt.Println("isClosed:", isClosed)
	//			return
	//	}
	//}
	for {
		if lm, ok := <-logMessages; ok {
			dispatch(lm)
		} else {
			break
		}
	}
	dispatcherClose()
}

func panicLoggerInstanceNotBeCreated() {
	panic("vlogger instance not be created, please call InitLoggerWithFile(fileName)")
}

func errorFunc(err error) {
	fmt.Println("vlog error:", err.Error())
}

func dispatcherClose() {
	vloggerInstance.lock.Lock()
	defer vloggerInstance.lock.Unlock()
	if !vloggerInstance.isClosed {
		err := vloggerInstance.disp.Close()
		if err != nil {
			errorFunc(err)
		}
		vloggerInstance.isClosed = true
	}
}

//==============================================================================

func InitLoggerWithFile(fileName string) (err error) {
	var config *configuration
	config, err = loadConfigurationFromFile(fileName)
	if err != nil {
		return err
	}
	vloggerInstance, err = getLoggerInstance(config)
	if err != nil {
		return err
	}
		
	logMessages = make(chan logMessage, 100)
	vloggerInstance.start()
	
	return nil
}

func Trace(params ...interface{}) {
	newLogMessage(LvTrace, params)
}

func Tracef(fmtString string, params ...interface{}) {

	newFormatLogMessage(LvTrace, fmtString, params)
}

func Debug(params ...interface{}) {
	newLogMessage(LvDebug, params)
}

func Debugf(fmtString string, params ...interface{}) {
	newFormatLogMessage(LvDebug, fmtString, params)
}

func Info(params ...interface{}) {
	newLogMessage(LvInfo, params)
}

func Infof(fmtString string, params ...interface{}) {
	newFormatLogMessage(LvInfo, fmtString, params)
}

func Warn(params ...interface{}) {
	newLogMessage(LvWarn, params)
}

func Warnf(fmtString string, params ...interface{}) {
	newFormatLogMessage(LvWarn, fmtString, params)
}

func Error(params ...interface{}) {
	newLogMessage(LvError, params)
}

func Errorf(fmtString string, params ...interface{}) {
	newFormatLogMessage(LvError, fmtString, params)
}

func Critical(params ...interface{}) {
	newLogMessage(LvCritical, params)
}

func Criticalf(fmtString string, params ...interface{}) {
	newFormatLogMessage(LvCritical, fmtString, params)
}

func writeRuntimeError(err error) {
	//不存在，则创建
	//存在，则打开附加写入
	logStr := fmt.Sprintf("%v: vlog runtime error %v\n", time.Now(), err)
	fileWriter, err := os.OpenFile(RUNTIME_ERROR_LOG_FILENAME,
		os.O_WRONLY|os.O_APPEND|os.O_CREATE, defaultFilePermissions)
	if err != nil {
		fmt.Print(logStr)
		return
	}
	defer fileWriter.Close()
	
	_, err = fileWriter.WriteString(logStr)
	if err != nil {
		fmt.Print(logStr)
	}
}

func Close() {
	for {
		select {
		case lm := <-logMessages:
			dispatch(lm)
		case <-time.After(5 * time.Second):
			close(logMessages)
			dispatcherClose()
			return
		}
	}
}