package vlog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultAutoIncrementNumDigit int           = 3
	DefaultAllowedFileMaxSize    int64         = 2 * 1024 * 1024 //2M
	DefaultOpenedFileMaxIdleTime time.Duration = time.Minute * 5
)

//用于自动编号的位数判定
var autoIncrementReg *regexp.Regexp

func init() {
	var err error
	autoIncrementReg, err = regexp.Compile("[#]+")
	if err != nil {
		err = errors.New("vlog error: fileWriter initialization error(" + err.Error() + ")")
		panic(err)
	}
}

type fileWriter struct {
	innerWriter           io.WriteCloser
	lock                  sync.Mutex
	fileName              string //日志文件名，绝对路径，含自动编号符号（如有）
	filePrefixName        string //用于自动编号，文件名前缀（自动编号符号前面部分），不含路径
	fileSuffixName        string //用于自动编号，文件名后缀（自动编号符号后面部分）
	autoIncrementNumDigit int    //用于自动编号，自动编号的位数，不足位数以零填充
	allowedMaxFileSize    int64  //允许的单个文件的最大字节数

	//以下属性应当在日志目录或文件创建成功后才赋值
	currentAbsPath              string //当前日志文件所在的目录，绝对路径
	currentStorageFileName      string //日志实际存储文件名，绝对路径
	currentFileSize             int64  //当前日志文件的字节数
	currentCountNumber          int    //当前自动编号计数器
	lastWriteTime               time.Time
	isNeedAutoFreeOpenedFile    bool
	lastAutoFreeOpenedFileTimer *time.Timer
}

func newFileWriter(fileName string, allowedMaxSize int64, isNeedAutoFreeOpenedFile bool) (writer *fileWriter, err error) {
	writer = new(fileWriter)
	writer.fileName = fileName
	if allowedMaxSize > 0 {
		writer.allowedMaxFileSize = allowedMaxSize
	} else {
		writer.allowedMaxFileSize = DefaultAllowedFileMaxSize
	}
	writer.isNeedAutoFreeOpenedFile = isNeedAutoFreeOpenedFile
	
	fileExtName := filepath.Ext(writer.fileName)

	autoIncrementSymbolIndexs := autoIncrementReg.FindIndex([]byte(writer.fileName))

	if len(autoIncrementSymbolIndexs) == 0 { //不含自动编号符号
		writer.autoIncrementNumDigit = DefaultAutoIncrementNumDigit
		writer.filePrefixName = strings.TrimSuffix(writer.fileName, fileExtName)
		writer.fileSuffixName = fileExtName
	} else { //含自动编号符号
		writer.autoIncrementNumDigit = autoIncrementSymbolIndexs[1] - autoIncrementSymbolIndexs[0]
		writer.filePrefixName = writer.fileName[:autoIncrementSymbolIndexs[0]]
		writer.fileSuffixName = writer.fileName[autoIncrementSymbolIndexs[1]:]
	}

	if strings.ContainsRune(writer.fileSuffixName, os.PathSeparator) {
		return nil, errors.New("folder can not be auto increment")
	}
	writer.filePrefixName = filepath.Base(writer.filePrefixName)
	return writer, nil
}

func (writer *fileWriter) Close() error {
	writer.lock.Lock()
	defer writer.lock.Unlock()
	if writer.innerWriter != nil {
		err := writer.innerWriter.Close()
		writer.innerWriter = nil
		return err
	}
	return nil
}

func (writer *fileWriter) autoFreeOpenedFile() {
	if writer.innerWriter != nil {
		nowTime := time.Now()
		expiredTime := writer.lastWriteTime.Add(DefaultOpenedFileMaxIdleTime)
		if nowTime.After(expiredTime) {
			//已过期，清理
			writer.Close()
		} else {
			//未过期，等会儿再检查
			writer.lastAutoFreeOpenedFileTimer = time.AfterFunc(DefaultOpenedFileMaxIdleTime,
				writer.autoFreeOpenedFile)
		}
	}
}

func (writer *fileWriter) Write(bytes []byte) (n int, err error) {
	writer.lastWriteTime = time.Now()
	//第一次写入数据
	if writer.currentStorageFileName == "" {
		//创建目录和文件
		err = writer.loadOrCreateStorageFileAndInitSomeAttributes()
		if err != nil {
			return 0, err
		}
	}

	//超过允许的大小，需新建文件
	if writer.currentFileSize >= writer.allowedMaxFileSize {
		writer.Close()
		writer.currentStorageFileName = writer.nextStorageFileName()
	}
	
	if writer.innerWriter == nil {
		writer.innerWriter, err = writer.newInnerWriter()
		if err != nil {
			return 0, nil
		}
	}

	n, err = writer.innerWriter.Write(bytes)
	if err == nil {
		//只在写入成功的情况下才累加字节数
		writer.currentFileSize += int64(n)
		writer.lastWriteTime = time.Now()
	}
	return n, err

}


func (writer *fileWriter) loadOrCreateStorageFileAndInitSomeAttributes() error {

	folder, _ := filepath.Split(writer.fileName)
	if len(folder) == 0 || !filepath.IsAbs(folder) {
		folder = workingDir + folder
	}
	
	_, err := os.Stat(folder)
	if err != nil {
		if os.IsNotExist(err) {
			//日志目录不存在，则创建目录
			err = os.MkdirAll(folder, defaultDirectoryPermissions)
			if err != nil {
				return err
			}
		} else {
			//无权限获取目录信息等其他情况
			return err
		}
	}
	writer.currentAbsPath = folder

	//获取日志文件名称
	writer.currentStorageFileName, err = writer.getStorageFileName()
	return err
}

//获取需要存储的日志文件的名称，主要用于启动时决定写入哪个文件
func (writer *fileWriter) getStorageFileName() (storageFileName string, err error) {
	//获取日志目录下文件名列表
	var logFiles []string = nil
	logFiles, err = getDirFilePaths(writer.currentAbsPath, nil, true)
	
	//logFiles为nil时会自动跳过，日志目录下没有任何文件
	filePrefixNameLength := len(writer.filePrefixName)
	for _, logFile := range logFiles {
		if strings.HasPrefix(logFile, writer.filePrefixName) {
			countStr := strings.TrimSuffix(logFile[filePrefixNameLength:],
				writer.fileSuffixName)
			countNumber, err := strconv.Atoi(countStr)
			if err != nil {
				continue
			}
			//获取最大编号和最大编号的文件
			if countNumber >= writer.currentCountNumber {
				writer.currentCountNumber = countNumber
				storageFileName = logFile
			}
		}
	}
	if storageFileName != "" {
		//检查最大编号文件的大小
		var maxNumberFileInfo os.FileInfo
		maxNumberFileInfo, err := os.Stat(writer.currentAbsPath + storageFileName)
		if err != nil {
			return "", err
		}
		maxNumberFileSize := maxNumberFileInfo.Size()
		if maxNumberFileSize < writer.allowedMaxFileSize {
			//未超出大小，继续使用
			writer.currentFileSize = maxNumberFileSize
		} else {
			//超出允许大小，下一个
			writer.currentCountNumber++
			storageFileName = writer.filePrefixName + 
				writer.getCountNumberSign() + writer.fileSuffixName
		}
	} else {
		//没有日志文件的情况
		storageFileName = writer.filePrefixName +
			writer.getCountNumberSign() + writer.fileSuffixName
	}
	return writer.currentAbsPath + storageFileName, nil
}

func (writer *fileWriter) getCountNumberSign() (sign string) {
	sign = strconv.Itoa(writer.currentCountNumber)
	length := len(sign)
	buf := bytes.NewBufferString("")
	for i := 0; i < writer.autoIncrementNumDigit-length; i++ {
		buf.WriteString("0")
	}
	buf.WriteString(sign)
	sign = buf.String()
	return sign
}

func (writer *fileWriter) newInnerWriter() (innerWriter io.WriteCloser, err error) {
	//打开日志文件，不存在则创建
	innerWriter, err = os.OpenFile(writer.currentStorageFileName, 
		os.O_WRONLY|os.O_APPEND|os.O_CREATE, defaultFilePermissions)
	if err == nil {
		writer.lastWriteTime = time.Now()
		
		if writer.isNeedAutoFreeOpenedFile {
			if writer.lastAutoFreeOpenedFileTimer != nil {
				writer.lastAutoFreeOpenedFileTimer = time.AfterFunc(DefaultOpenedFileMaxIdleTime,
					writer.autoFreeOpenedFile)
			}
		}
	}
	return innerWriter, err
}

func (writer *fileWriter) nextStorageFileName() string {
	writer.currentCountNumber++
	writer.currentFileSize = 0
	return writer.currentAbsPath + writer.filePrefixName +
		writer.getCountNumberSign() + writer.fileSuffixName
}

func (writer *fileWriter) String() string {
	return "fileWriter: fileName=" + writer.fileName +
		", filePrefixName=" + writer.filePrefixName +
		", fileSuffixName=" + writer.fileSuffixName +
		", autoIncrementNumDigit=" + fmt.Sprint(writer.autoIncrementNumDigit) +
		", allowedMaxFileSize=" + fmt.Sprint(writer.allowedMaxFileSize) +
		", currentAbsPath=" + writer.currentAbsPath +
		", currentStorageFileName=" + writer.currentStorageFileName +
		", currentFileSize=" + fmt.Sprint(writer.currentFileSize) +
		", currentCountNumber=" + fmt.Sprint(writer.currentCountNumber) +
		", lastWriterTime=" + fmt.Sprint(writer.lastWriteTime) +
		", isNeedAutoFreeOpenedFile=" + fmt.Sprint(writer.isNeedAutoFreeOpenedFile)
}