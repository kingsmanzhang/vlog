package vlog

import (
	"errors"
	"os"
	"strconv"
	"strings"

	xml "tangacg.com/xmlnode"
)

type configuration struct {
	writersNode    *xml.Node
	formattersNode *xml.Node
	maxLevel       LogLevel
	minLevel       LogLevel
	writers        []*formattedWriter
	formatters     map[string]*formatter
}

func loadConfigurationFromFile(fileName string) (config *configuration, err error) {

	var file *os.File
	file, err = os.OpenFile(fileName, os.O_RDONLY, defaultFilePermissions)
	if err != nil {
		return nil, err
	}

	var rootNode *xml.Node
	rootNode, err = xml.UnmarshalConfig(file)
	if err != nil {
		return nil, err
	}
	if rootNode.Name != "vlog" || !rootNode.HasChildren() {
		return nil, errors.New("config file err: the root element is not vlog or vlog element has no child element. at file " + fileName)
	}

	config = new(configuration)
	config.maxLevel = LvCritical
	config.minLevel = LvTrace

	minLevelStr, ok := rootNode.Attributes["minlevel"]
	if ok {
		level, isValid := lv4StringMap[minLevelStr]
		if !isValid {
			return nil, errors.New(rootNode.Name + "'s attribute minlevel value is llegal: " + minLevelStr + ".")
		}
		config.minLevel = level
	}

	maxLevelStr, ok := rootNode.Attributes["maxlevel"]
	if ok {
		level, isValid := lv4StringMap[maxLevelStr]
		if !isValid {
			return nil, errors.New(rootNode.Name + "'s attribute maxlevel value is llegal: " + maxLevelStr + ".")
		}
		config.maxLevel = level
	}

	for _, elt := range rootNode.Children {
		switch elt.Name {
		case "outputters":
			if config.writersNode != nil {
				return nil, errors.New("there must be only one outputter element.")
			}
			config.writersNode = elt
		case "formatters":
			if config.formattersNode != nil {
				return nil, errors.New("there must be only one outputter element.")
			}
			config.formattersNode = elt
		default:
			return nil, errors.New("there was a unallowed element " + elt.String() + ".")
		}
	}

	if config.writersNode == nil {
		return nil, errors.New("there was no outputters element.")
	}
	if config.formattersNode == nil {
		return nil, errors.New("there was no formatters element.")
	}

	err = config.initFormatters()
	if err != nil {
		return nil, err
	}
	if !config.formattersNode.HasChildren() || len(config.formatters) == 0 {
		return nil, errors.New("formatters element must have one child element at least named formatter.")
	}
	err = config.initWrites()
	if err != nil {
		return nil, err
	}
	if !config.writersNode.HasChildren() || len(config.writers) == 0 {
		return nil, errors.New("outputters element must have one child element at least.")
	}
	return config, nil
}

func (config *configuration) initWrites() (err error) {
	config.writers = make([]*formattedWriter, 0)
	for _, elt := range config.writersNode.Children {
		var writer *formattedWriter
		switch elt.Name {
		case "rulefile":
			writer, err = config.newRuleFileFormattedWriterByXMLNode(elt)
			if err != nil {
				return err
			}
		case "file":
			writer, err = config.newFileFormattedWriterByXMLNode(elt)
			if err != nil {
				return err
			}
		case "console":
			writer, err = config.newConsoleFormattedWriterByXMLNode(elt)
			if err != nil {
				return err
			}
		case "database":
			writer, err = config.newDatabaseFormattedWriterByXMLNode(elt)
			if err != nil {
				return err
			}
		default:
			return errors.New("there was a unallowed element " + elt.String() + ".")
		}
		config.writers = append(config.writers, writer)
	}
	return nil
}

func (config *configuration) initFormatters() (err error) {
	config.formatters = make(map[string]*formatter, 0)
	for _, elt := range config.formattersNode.Children {
		var formatterID string
		var formatter *formatter
		if elt.Name == "formatter" {
			formatterID, formatter, err = newFormatterByXMLNode(elt)
			if err != nil {
				config.formatters = nil
				return err
			}
		} else {
			return errors.New("there was a unallowed element " + elt.String() + ".")
		}
		config.formatters[formatterID] = formatter
	}
	return nil
}

func newFormatterByXMLNode(node *xml.Node) (formatterID string, formatter *formatter, err error) {
	fmtID, ok := node.Attributes["id"]
	if !ok {
		return "", nil, errors.New(node.Name + " must have id attribute")
	}
	fmtString, ok := node.Attributes["format"]
	if !ok {
		return "", nil, errors.New(node.Name + " must have format attribute")
	}
	formatter, err = newFormatter(fmtString, nil)
	if err != nil {
		return "", nil, err
	}
	formatterID = fmtID
	return formatterID, formatter, nil
}

func (config *configuration) newConsoleFormattedWriterByXMLNode(node *xml.Node) (writer *formattedWriter, err error) {
	_, formatterid, allowedLevelList, _, err := parseNodeAttrToWriterInfo(node)
	if err != nil {
		return nil, err
	}
	formatter, ok := config.formatters[formatterid]
	if !ok {
		return nil, errors.New("there was no formatter the id by " + formatterid)
	}

	var cw *consoleWriter
	cw, err = newConsoleWriter()
	if err != nil {
		return nil, err
	}
	writer, err = newFormattedWriter(cw, formatter, allowedLevelList)
	if err != nil {
		return nil, err
	}
	return writer, nil
}

func (config *configuration) newFileFormattedWriterByXMLNode(node *xml.Node) (writer *formattedWriter, err error) {
	fileName, formatterid, allowedLevelList, maxSize, err := parseNodeAttrToWriterInfo(node)
	if err != nil {
		return nil, err
	}
	if fileName == "" {
		return nil, errors.New(node.Name + " element has no filename attribute")
	}
	formatter, ok := config.formatters[formatterid]
	if !ok {
		return nil, errors.New("there was no formatter the id by " + formatterid)
	}
	var fw *fileWriter
	fw, err = newFileWriter(fileName, maxSize, true)
	if err != nil {
		return nil, err
	}

	writer, err = newFormattedWriter(fw, formatter, allowedLevelList)
	if err != nil {
		return nil, err
	}
	return writer, nil
}

func (config *configuration) newRuleFileFormattedWriterByXMLNode(node *xml.Node) (writer *formattedWriter, err error) {
	fileName, formatterid, allowedLevelList, maxSize, err := parseNodeAttrToWriterInfo(node)
	if err != nil {
		return nil, err
	}
	if fileName == "" {
		return nil, errors.New(node.Name + " element has no filename attribute")
	}
	formatter, ok := config.formatters[formatterid]
	if !ok {
		return nil, errors.New("there was no formatter the id by " + formatterid)
	}
	var rfw *ruleFileWriter
	rfw, err = newRuleFileWriter(fileName, maxSize)
	if err != nil {
		return nil, err
	}
	writer, err = newFormattedWriter(rfw, formatter, allowedLevelList)
	if err != nil {
		return nil, err
	}
	return writer, nil
}

func (config *configuration) newDatabaseFormattedWriterByXMLNode(node *xml.Node) (writer *formattedWriter, err error) {
	dbType, connUrl, tableName, formatterid, allowedLevelList, err := parseNodeAttrToDBWriterInfo(node)
	formatter, ok := config.formatters[formatterid]
	if !ok {
		return nil, errors.New("there was no formatter the id by " + formatterid)
	}
	var dbWriter *databaseWriter
	dbWriter, err = newDababaseWriter(dbType, connUrl, tableName)
	if err != nil {
		return nil, err
	}
	writer, err = newFormattedWriter(dbWriter, formatter, allowedLevelList)
	if err != nil {
		return nil, err
	}
	return writer, nil
}

func parseNodeAttrToDBWriterInfo(node *xml.Node) (dbType, connUrl, tableName, formatterid string,
	allowedLevelList map[LogLevel]bool, err error) {
	var ok bool = false
	dbType, ok = node.Attributes["type"]
	if !ok {
		return dbType, connUrl, tableName, formatterid, allowedLevelList,
			errors.New(node.Name + " must be have type attribute.")
	}
	connUrl, ok = node.Attributes["connurl"]
	if !ok {
		return dbType, connUrl, tableName, formatterid, allowedLevelList,
			errors.New(node.Name + " must be have connurl attribute.")
	}
	tableName, ok = node.Attributes["tablename"]
	if !ok {
		return dbType, connUrl, tableName, formatterid, allowedLevelList,
			errors.New(node.Name + " must be have tablename attribute.")
	}
	formatterid, ok = node.Attributes["formatterid"]
	if !ok {
		return dbType, connUrl, tableName, formatterid, allowedLevelList,
			errors.New(node.Name + " must be have formatterid attribute.")
	}
	allowedLevelList = parseAllowedLevelList(node)
	return dbType, connUrl, tableName, formatterid, allowedLevelList, nil
}

func parseNodeAttrToWriterInfo(node *xml.Node) (fileName, formatterid string,
	allowedLevelList map[LogLevel]bool, maxSize int64, err error) {
	var ok bool = false
	fileName, ok = node.Attributes["filename"]
	formatterid, ok = node.Attributes["formatterid"]
	if !ok {
		return "", "", nil, 0, errors.New(node.Name + " must have formatterid attribute.")
	}
	if maxSizeStr, ok := node.Attributes["maxsize"]; ok {
		maxSize, err = strconv.ParseInt(maxSizeStr, 10, 64)
		if err != nil {
			return "", "", nil, 0, errors.New(node.Name + "'s attribute maxsize value is illegal: " + err.Error())
		}
	}
	if maxSize <= 0 {
		maxSize = DefaultAllowedFileMaxSize
	}
	allowedLevelList = parseAllowedLevelList(node)
	return fileName, formatterid, allowedLevelList, maxSize, nil
}

func parseAllowedLevelList(node *xml.Node) (allowedLevelList map[LogLevel]bool) {
	allowedLevelList = map[LogLevel]bool{
		LvTrace:    false,
		LvDebug:    false,
		LvInfo:     false,
		LvWarn:     false,
		LvError:    false,
		LvCritical: false,
	}
	levels, ok := node.Attributes["levels"]
	if ok {
		levelsSlice := strings.Split(levels, ",")
		for _, v := range levelsSlice {
			level, ok := lv4StringMap[v]
			if ok {
				allowedLevelList[level] = true
			}
		}
	} else {
		//如果未配置levels属性，则允许全部等级
		for level, _ := range allowedLevelList {
			allowedLevelList[level] = true
		}
	}
	return allowedLevelList
}
