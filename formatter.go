package vlog

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	TagSymbol = '%'
)
const (
	tagSymbolString = "%"
	tagParamStart   = '('
	tagParamEnd     = ')'
)

// These are the time and date formats that are used when %Date or %Time format aliases are used.
const (
	DateFormat = "2006-01-02"
	TimeFormat = "15:04:05"
)

var DefaultMsgFormat = "%ns [%level] %msg%n"

var defaultFormatter *formatter
var msgOnlyFormatter *formatter

func init() {
	var err error
	defaultFormatter, err = newFormatter(DefaultMsgFormat, nil)
	if err != nil {
		fmt.Println("Error during defaultFormatter creation: " + err.Error())
	}
	msgOnlyFormatter, err = newFormatter("%msg", nil)
	if err != nil {
		fmt.Println("Error during msgOnlyFormatter creation: " + err.Error())
	}
}

type tagFunc func(message string, level LogLevel, context runtimeContextInterface) interface{}
type tagFuncCreator func(param string) tagFunc

var tagFuncs = map[string]tagFunc{
	"level":   tagLevel,
	"lv":      tagLv,
	"LV":      tagLV,
	"msg":     tagMsg,
	"file":    tagFile,
	"relfile": tagRelFile,
	"func":    tagFunction,
	"fn":      tagFunctionShort,
	"line":    tagLine,
	"time":    tagTime,
	"ns":      tagNs,
	"n":       tagN,
	"t":       tagT,
}

var tagWithParamFuncCreator = map[string]tagFuncCreator{
	"date": createDateTimeTagFunc,
	"escm": createANSIEscapeFunc,
}

type formatter struct {
	//带标签的格式化字符串
	fmtStringOriginal string
	fmtString         string
	//允许的标签列表length为零或nil表示允许全部支持的标签
	allowedTags []string
	//标签处理函数
	tagFuncs    []tagFunc
}

// NewFormatter参数：
// formatString: 带标签的格式化字符串
// allowedTags: 允许的标签列表，length为零或nil表示允许全部支持的标签
func newFormatter(formatString string, allowedTags []string) (*formatter, error) {
	newformatter := new(formatter)
	newformatter.fmtStringOriginal = formatString
	newformatter.allowedTags = allowedTags

	err := newformatter.buildTagFuncs()
	if err != nil {
		return nil, err
	}

	return newformatter, nil
}

func (formatter *formatter) buildTagFuncs() error {
	formatter.tagFuncs = make([]tagFunc, 0)
	var fmtString string
	for i := 0; i < len(formatter.fmtStringOriginal); i++ {
		char := formatter.fmtStringOriginal[i]
		if char != TagSymbol {
			fmtString += string(char)
			continue
		}

		isEndOfStr := i == len(formatter.fmtStringOriginal)-1
		if isEndOfStr {
			return errors.New(fmt.Sprintf("Format error: %v - last symbol", tagSymbolString))
		}

		isDoubledVerbSymbol := formatter.fmtStringOriginal[i+1] == TagSymbol
		if isDoubledVerbSymbol {
			fmtString += tagSymbolString
			i++
			continue
		}

		function, nextI, err := formatter.extractTagFunc(i + 1)
		if err != nil {
			return err
		}

		fmtString += "%v"
		i = nextI
		formatter.tagFuncs = append(formatter.tagFuncs, function)
	}

	formatter.fmtString = fmtString
	return nil
}

func (formatter *formatter) extractTagFunc(index int) (tagFunc, int, error) {
	letterSequence := formatter.extractLetterSequence(index)
	if len(letterSequence) == 0 {
		return nil, 0, errors.New(fmt.Sprintf("Format error: lack of tag after %v. At %v", tagSymbolString, index))
	}

	if formatter.allowedTags != nil && len(formatter.allowedTags) != 0 {
		allowed := false
		for _, v := range formatter.allowedTags {
			if letterSequence == v {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, 0, errors.New("Format error: unallowed tag at " + strconv.Itoa(index) + ": " + letterSequence)
		}
	}

	function, tagLength, ok := formatter.findTagFunc(letterSequence)
	if ok {
		return function, index + tagLength - 1, nil
	}

	function, tagLength, ok = formatter.findTagFuncParametrized(letterSequence, index)
	if ok {
		return function, index + tagLength - 1, nil
	}

	return nil, 0, errors.New("Format error: unrecognized tag at " + strconv.Itoa(index) + ": " + letterSequence)
}

func (formatter *formatter) extractLetterSequence(index int) string {
	letters := ""

	bytesToParse := []byte(formatter.fmtStringOriginal[index:])
	runeCount := utf8.RuneCount(bytesToParse)
	for i := 0; i < runeCount; i++ {
		rune, runeSize := utf8.DecodeRune(bytesToParse)
		bytesToParse = bytesToParse[runeSize:]

		if unicode.IsLetter(rune) {
			letters += string(rune)
		} else {
			break
		}
	}
	return letters
}

func (formatter *formatter) findTagFunc(letters string) (tagFunc, int, bool) {
	currentTag := letters
	for i := 0; i < len(letters); i++ {
		function, ok := tagFuncs[currentTag]
		if ok {
			return function, len(currentTag), ok
		}
		currentTag = currentTag[:len(currentTag)-1]
	}

	return nil, 0, false
}

func (formatter *formatter) findTagFuncParametrized(letters string, lettersStartIndex int) (tagFunc, int, bool) {
	currentTag := letters
	for i := 0; i < len(letters); i++ {
		functionCreator, ok := tagWithParamFuncCreator[currentTag]
		if ok {
			paramter := ""
			parameterLen := 0
			isVerbEqualsLetters := i == 0 // if not, then letter goes after tag, and tag is parameterless
			if isVerbEqualsLetters {
				userParamter := ""
				userParamter, parameterLen, ok = formatter.findparameter(lettersStartIndex + len(currentTag))
				if ok {
					paramter = userParamter
				}
			}

			return functionCreator(paramter), len(currentTag) + parameterLen, true
		}

		currentTag = currentTag[:len(currentTag)-1]
	}

	return nil, 0, false
}

func (formatter *formatter) findparameter(startIndex int) (string, int, bool) {
	if len(formatter.fmtStringOriginal) == startIndex || formatter.fmtStringOriginal[startIndex] != tagParamStart {
		return "", 0, false
	}

	endIndex := strings.Index(formatter.fmtStringOriginal[startIndex:], string(tagParamEnd)) + startIndex
	if endIndex == -1 {
		return "", 0, false
	}

	length := endIndex - startIndex + 1

	return formatter.fmtStringOriginal[startIndex+1 : endIndex], length, true
}

func (formatter *formatter) Format(message string, level LogLevel, context runtimeContextInterface) string {
	if len(formatter.tagFuncs) == 0 {
		return formatter.fmtString
	}

	params := make([]interface{}, len(formatter.tagFuncs))
	for i, function := range formatter.tagFuncs {
		params[i] = function(message, level, context)
	}

	return fmt.Sprintf(formatter.fmtString, params...)
}

func (formatter *formatter) String() string {
	return formatter.fmtStringOriginal
}

//=====================================================

const (
	wrongLogLevel   = "WRONG_LOGLEVEL_TAG"
	wrongEscapeCode = "WRONG_CONSLE_ESCAPE"
	wrongNumDigit   = "WRONG_NUM_DIGIT"
)

//%level
func tagLevel(message string, level LogLevel, context runtimeContextInterface) interface{} {
	levelStr, ok := lv2StringMap[level]
	if !ok {
		return wrongLogLevel
	}
	return levelStr
}

//%lv
func tagLv(message string, level LogLevel, context runtimeContextInterface) interface{} {
	return strings.ToLower(tagLV(message, level, context).(string))
}

//%LV
func tagLV(message string, level LogLevel, context runtimeContextInterface) interface{} {
	levelStr, ok := lv2StrMap[level]
	if !ok {
		return wrongLogLevel
	}
	return levelStr
}

//%msg
func tagMsg(message string, level LogLevel, context runtimeContextInterface) interface{} {
	return message
}

//%file
func tagFile(message string, level LogLevel, context runtimeContextInterface) interface{} {
	return context.FileName()
}

//%relfile
func tagRelFile(message string, level LogLevel, context runtimeContextInterface) interface{} {
	return context.FullPath()
}

//%func
func tagFunction(message string, level LogLevel, context runtimeContextInterface) interface{} {
	return context.Func()
}

//%fn
func tagFunctionShort(message string, level LogLevel, context runtimeContextInterface) interface{} {
	f := context.Func()
	spl := strings.Split(f, ".")
	return spl[len(spl)-1]
}

//%line
func tagLine(message string, level LogLevel, context runtimeContextInterface) interface{} {
	return context.Line()
}

//%time
func tagTime(message string, level LogLevel, context runtimeContextInterface) interface{} {
	return context.CallTime().Format(TimeFormat)
}

//%ns
func tagNs(message string, level LogLevel, context runtimeContextInterface) interface{} {
	return context.CallTime().UnixNano()
}

//%n
func tagN(message string, level LogLevel, context runtimeContextInterface) interface{} {
	return "\n"
}

//%t
func tagT(message string, level LogLevel, context runtimeContextInterface) interface{} {
	return "\t"
}

//%date("format pattern")
func createDateTimeTagFunc(dateTimeFormat string) tagFunc {
	format := dateTimeFormat
	if format == "" {
		format = DateFormat
	}
	return func(message string, level LogLevel, context runtimeContextInterface) interface{} {
		return time.Now().Format(format)
	}
}

//%escm[n]仅用于控制台的颜色输出
func createANSIEscapeFunc(escapeCodeString string) tagFunc {
	return func(message string, level LogLevel, context runtimeContextInterface) interface{} {
		if len(escapeCodeString) == 0 {
			return wrongEscapeCode
		}

		return fmt.Sprintf("%c[%sm", 0x1B, escapeCodeString)
	}
}
