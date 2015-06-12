package vlog

//日志等级类型
type LogLevel int8

//日志等级常量
const (
	LvTrace = iota
	LvDebug
	LvInfo
	LvWarn
	LvError
	LvCritical
	lvImportant //需要输出到数据库的日志详细信息
)

//日志等级字符串表示
const (
	lvTraceString     = "trace"
	lvDebugString     = "debug"
	lvInfoString      = "info"
	lvWarnString      = "warn"
	lvErrorString     = "error"
	lvCriticalString  = "critical"
	//lvImportantString = "important"

	lvTraceStr     = "TRA"
	lvDebugStr     = "DEB"
	lvInfoStr      = "INF"
	lvWarnStr      = "WAR"
	lvErrorStr     = "ERR"
	lvCriticalStr  = "CRI"
	//lvImportantStr = "IMP"
)

//日志等级字符串表示 -> 日志等级
var lv4StringMap = map[string]LogLevel{
	lvTraceString:     LvTrace,
	lvDebugString:     LvDebug,
	lvInfoString:      LvInfo,
	lvWarnString:      LvWarn,
	lvErrorString:     LvError,
	lvCriticalString:  LvCritical,
	//lvImportantString: lvImportant,
}

//日志等级 -> 日志等级字符串表示
var lv2StringMap = map[LogLevel]string{
	LvTrace:     lvTraceString,
	LvDebug:     lvDebugString,
	LvInfo:      lvInfoString,
	LvWarn:      lvWarnString,
	LvError:     lvErrorString,
	LvCritical:  lvCriticalString,
	//lvImportant: lvImportantString,
}

//日志等级 -> 日志等级短字符串表示
var lv2StrMap = map[LogLevel]string{
	LvTrace:     lvTraceStr,
	LvDebug:     lvDebugStr,
	LvInfo:      lvInfoStr,
	LvWarn:      lvWarnStr,
	LvError:     lvErrorStr,
	LvCritical:  lvCriticalStr,
	//lvImportant: lvImportantStr,
}

//返回日志等级字符串表示，方便打印
func (lv LogLevel) String() string {
	lvString, ok := lv2StringMap[lv]
	if ok {
		return lvString
	}
	return ""
}

//返回日志等级短字符串表示，方便打印
func (lv LogLevel) ShortString() string {
	lvStr, ok := lv2StrMap[lv]
	if ok {
		return lvStr
	}
	return ""
}
