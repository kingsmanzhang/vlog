package vlog

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

type ruleFileWriter struct {
	io.WriteCloser
	lock               sync.Mutex
	fileNameFormatter  *formatter //文件名格式化器
	fileName           string     //每次写入之前格式化的文件名（含自动编号符号）
	allowedMaxFileSize int64      //单个文件允许的最大字节数
	//真正写入日志信息的fileWriter fileWriter.fileName => *fileWriter
	fileWriters map[string]*fileWriter

	isNeedAutoFreeOpenedFileWriters    bool
	lastAutoFreeOpenedFileWritersTimer *time.Timer
}

func newRuleFileWriter(fileNameOriginal string, maxSize int64) (writer *ruleFileWriter, err error) {
	writer = new(ruleFileWriter)
	writer.fileNameFormatter, err = newFormatter(fileNameOriginal,
		[]string{"date", "level", "LV", "lv"})
	if err != nil {
		return nil, err
	}
	writer.allowedMaxFileSize = maxSize
	//必须为true
	writer.isNeedAutoFreeOpenedFileWriters = true
	writer.fileWriters = make(map[string]*fileWriter, 0)
	return writer, nil
}

//每次写入前都应调用此方法
func (writer *ruleFileWriter) formatFileName(level LogLevel, context runtimeContextInterface) {
	writer.fileName = writer.fileNameFormatter.Format("", level, context)
}

func (writer *ruleFileWriter) autoFreeOpenedFileWriters() {
	nowTime := time.Now()
	for fileName, fWriter := range writer.fileWriters {
		if fWriter == nil || fWriter.innerWriter == nil {
			continue
		}
		expiredTime := fWriter.lastWriteTime.Add(DefaultOpenedFileMaxIdleTime)
		if nowTime.After(expiredTime) {
			writer.closeFileWriter(fileName, fWriter)
		}
	}

	//如果已全部清理完毕，则不再检查
	//直到又一个新的fileWriter作为第一个加入时再次开启
	if len(writer.fileWriters) != 0 {
		writer.lastAutoFreeOpenedFileWritersTimer = time.AfterFunc(DefaultOpenedFileMaxIdleTime,
			writer.autoFreeOpenedFileWriters)
	}
}

func (writer *ruleFileWriter) Write(bytes []byte) (n int, err error) {
	
	//用于决定是否需要开启（或重新开启）autoFreeOpenedFileWriter()
	//innerFileWriterCount为零时开启
	//当且仅当在空的fileWriters map中新加入一个fileWriter时才开启
	innerFileWriterCount := len(writer.fileWriters)

	fWriter, ok := writer.fileWriters[writer.fileName]
	if !ok {
		fWriter, err = newFileWriter(writer.fileName,
			writer.allowedMaxFileSize, false)
		if err != nil {
			return 0, err
		}
		writer.fileWriters[writer.fileName] = fWriter

		if writer.isNeedAutoFreeOpenedFileWriters && innerFileWriterCount == 0 {
			//=====已无必要，因为只有当lastFreeFreeOpenedFileWriterTimer未开启或结束了
			//=====innerFileWriterCount才有可能为零
			//新开启前关闭之前的timer
			//if writer.lastAutoFreeOpenedFileWritersTimer != nil {
			//	writer.lastAutoFreeOpenedFileWritersTimer.Stop()
			//}
			//=====已无必要 end
			writer.lastAutoFreeOpenedFileWritersTimer = time.AfterFunc(DefaultOpenedFileMaxIdleTime,
				writer.autoFreeOpenedFileWriters)
		}
	}
	n, err = fWriter.Write(bytes)
	return n, err
}

func (writer *ruleFileWriter) closeFileWriter(fileName string, fWriter *fileWriter) error {
	writer.lock.Lock()
	defer writer.lock.Unlock()
	//无论是否成功关闭，这个fileWriter是不能再使用了（原则）
	delete(writer.fileWriters, fileName)
	if fWriter.innerWriter != nil {
		err := fWriter.innerWriter.Close()
		fWriter.innerWriter = nil
		return err
	}
	return nil
}

func (writer *ruleFileWriter) Close() (err error) {
	writer.lock.Lock()
	defer writer.lock.Unlock()
	errMsg := ""
	for fileName, fileWriter := range writer.fileWriters {
		delete(writer.fileWriters, fileName)
		err := fileWriter.innerWriter.Close()
		fileWriter.innerWriter = nil
		if err != nil {
			errMsg += fileName + " closed error: " + err.Error() + ","
		}
	}
	writer.fileWriters = nil
	if errMsg != "" {
		return errors.New("some fileWriter closed error: [" +
			errMsg[:len(errMsg)-1] + "]")
	}
	return nil
}

func (writer *ruleFileWriter) String() string {
	return "ruleFileWriter: fileNameFormatter=(" + writer.fileNameFormatter.String() + ")" +
		", allowedMaxSize=" + fmt.Sprint(writer.allowedMaxFileSize) +
		", lastWriteFileName=" + writer.fileName +
		", fileWriters=[count:" + fmt.Sprint(len(writer.fileWriters)) + "](" +
		fmt.Sprintf("%v)", writer.fileWriters)
}
