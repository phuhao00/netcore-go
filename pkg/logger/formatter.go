package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// TextFormatter 文本格式化器
type TextFormatter struct {
	// TimestampFormat 时间戳格式
	TimestampFormat string
	// DisableTimestamp 禁用时间戳
	DisableTimestamp bool
	// DisableColors 禁用颜色
	DisableColors bool
	// FullTimestamp 使用完整时间戳
	FullTimestamp bool
	// DisableLevelTruncation 禁用级别截断
	DisableLevelTruncation bool
	// PadLevelText 填充级别文本
	PadLevelText bool
	// QuoteEmptyFields 引用空字段
	QuoteEmptyFields bool
	// FieldMap 字段映射
	FieldMap map[string]string
	// CallerPrettyfier 调用者美化器
	CallerPrettyfier func(*CallerInfo) (function string, file string)
}

// Format 格式化日志条目
func (f *TextFormatter) Format(entry *Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Fields != nil {
		b = &bytes.Buffer{}
	} else {
		b = &bytes.Buffer{}
	}
	
	// 时间戳
	if !f.DisableTimestamp {
		timestampFormat := f.TimestampFormat
		if timestampFormat == "" {
			if f.FullTimestamp {
				timestampFormat = time.RFC3339
			} else {
				timestampFormat = "2006-01-02 15:04:05"
			}
		}
		b.WriteString(entry.Time.Format(timestampFormat))
		b.WriteByte(' ')
	}
	
	// 日志级别
	levelText := f.getLevelText(entry.Level)
	if f.PadLevelText {
		levelText = fmt.Sprintf("%-5s", levelText)
	}
	
	if !f.DisableColors {
		levelText = f.getColoredLevel(entry.Level, levelText)
	}
	
	b.WriteString(levelText)
	b.WriteByte(' ')
	
	// 调用者信息
	if entry.Caller != nil {
		if f.CallerPrettyfier != nil {
			function, file := f.CallerPrettyfier(entry.Caller)
			if function != "" {
				b.WriteString(function)
				b.WriteByte(' ')
			}
			if file != "" {
				b.WriteString(file)
				b.WriteByte(' ')
			}
		} else {
			b.WriteString(fmt.Sprintf("%s:%d ", entry.Caller.File, entry.Caller.Line))
		}
	}
	
	// 消息
	b.WriteString(entry.Message)
	
	// 错误信息
	if entry.Error != "" {
		b.WriteString(" error=")
		b.WriteString(f.quoteString(entry.Error))
	}
	
	// 字段
	if len(entry.Fields) > 0 {
		f.appendFields(b, entry.Fields)
	}
	
	b.WriteByte('\n')
	return b.Bytes(), nil
}

// getLevelText 获取级别文本
func (f *TextFormatter) getLevelText(level Level) string {
	if f.DisableLevelTruncation {
		return level.String()
	}
	
	switch level {
	case TraceLevel:
		return "TRAC"
	case DebugLevel:
		return "DEBU"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERRO"
	case FatalLevel:
		return "FATA"
	case PanicLevel:
		return "PANI"
	default:
		return "UNKN"
	}
}

// getColoredLevel 获取彩色级别文本
func (f *TextFormatter) getColoredLevel(level Level, levelText string) string {
	switch level {
	case TraceLevel:
		return fmt.Sprintf("\x1b[37m%s\x1b[0m", levelText) // 白色
	case DebugLevel:
		return fmt.Sprintf("\x1b[36m%s\x1b[0m", levelText) // 青色
	case InfoLevel:
		return fmt.Sprintf("\x1b[32m%s\x1b[0m", levelText) // 绿色
	case WarnLevel:
		return fmt.Sprintf("\x1b[33m%s\x1b[0m", levelText) // 黄色
	case ErrorLevel:
		return fmt.Sprintf("\x1b[31m%s\x1b[0m", levelText) // 红色
	case FatalLevel:
		return fmt.Sprintf("\x1b[35m%s\x1b[0m", levelText) // 紫色
	case PanicLevel:
		return fmt.Sprintf("\x1b[41m%s\x1b[0m", levelText) // 红色背景
	default:
		return levelText
	}
}

// appendFields 追加字段
func (f *TextFormatter) appendFields(b *bytes.Buffer, fields map[string]interface{}) {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	for _, key := range keys {
		value := fields[key]
		b.WriteByte(' ')
		
		// 字段映射
		if f.FieldMap != nil {
			if mappedKey, ok := f.FieldMap[key]; ok {
				key = mappedKey
			}
		}
		
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(f.formatValue(value))
	}
}

// formatValue 格式化值
func (f *TextFormatter) formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return f.quoteString(v)
	case error:
		return f.quoteString(v.Error())
	default:
		return fmt.Sprintf("%v", v)
	}
}

// quoteString 引用字符串
func (f *TextFormatter) quoteString(s string) string {
	if f.QuoteEmptyFields && s == "" {
		return `""`
	}
	
	if strings.ContainsAny(s, " \t\n\r") {
		return fmt.Sprintf(`"%s"`, strings.ReplaceAll(s, `"`, `\"`))
	}
	return s
}

// JSONFormatter JSON格式化器
type JSONFormatter struct {
	// TimestampFormat 时间戳格式
	TimestampFormat string
	// DisableTimestamp 禁用时间戳
	DisableTimestamp bool
	// DisableHTMLEscape 禁用HTML转义
	DisableHTMLEscape bool
	// DataKey 数据键名
	DataKey string
	// FieldMap 字段映射
	FieldMap map[string]string
	// CallerPrettyfier 调用者美化器
	CallerPrettyfier func(*CallerInfo) (function string, file string)
	// PrettyPrint 美化打印
	PrettyPrint bool
}

// Format 格式化日志条目
func (f *JSONFormatter) Format(entry *Entry) ([]byte, error) {
	data := make(map[string]interface{})
	
	// 时间戳
	if !f.DisableTimestamp {
		timestampFormat := f.TimestampFormat
		if timestampFormat == "" {
			timestampFormat = time.RFC3339Nano
		}
		data[f.getFieldKey("time")] = entry.Time.Format(timestampFormat)
	}
	
	// 日志级别
	data[f.getFieldKey("level")] = entry.Level.String()
	
	// 消息
	data[f.getFieldKey("msg")] = entry.Message
	
	// 错误信息
	if entry.Error != "" {
		data[f.getFieldKey("error")] = entry.Error
	}
	
	// 调用者信息
	if entry.Caller != nil {
		if f.CallerPrettyfier != nil {
			function, file := f.CallerPrettyfier(entry.Caller)
			if function != "" {
				data[f.getFieldKey("func")] = function
			}
			if file != "" {
				data[f.getFieldKey("file")] = file
			}
		} else {
			data[f.getFieldKey("file")] = fmt.Sprintf("%s:%d", entry.Caller.File, entry.Caller.Line)
			data[f.getFieldKey("func")] = entry.Caller.Function
		}
	}
	
	// 字段
	for k, v := range entry.Fields {
		key := f.getFieldKey(k)
		data[key] = v
	}
	
	var jsonData []byte
	var err error
	
	if f.PrettyPrint {
		jsonData, err = json.MarshalIndent(data, "", "  ")
	} else {
		jsonData, err = json.Marshal(data)
	}
	
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fields to JSON: %w", err)
	}
	
	if !f.DisableHTMLEscape {
		// JSON默认会转义HTML字符，这里保持默认行为
	}
	
	return append(jsonData, '\n'), nil
}

// getFieldKey 获取字段键名
func (f *JSONFormatter) getFieldKey(key string) string {
	if f.FieldMap != nil {
		if mappedKey, ok := f.FieldMap[key]; ok {
			return mappedKey
		}
	}
	return key
}

// LogfmtFormatter Logfmt格式化器
type LogfmtFormatter struct {
	// TimestampFormat 时间戳格式
	TimestampFormat string
	// DisableTimestamp 禁用时间戳
	DisableTimestamp bool
	// FieldMap 字段映射
	FieldMap map[string]string
}

// Format 格式化日志条目
func (f *LogfmtFormatter) Format(entry *Entry) ([]byte, error) {
	var b bytes.Buffer
	
	// 时间戳
	if !f.DisableTimestamp {
		timestampFormat := f.TimestampFormat
		if timestampFormat == "" {
			timestampFormat = time.RFC3339
		}
		b.WriteString("time=")
		b.WriteString(entry.Time.Format(timestampFormat))
		b.WriteByte(' ')
	}
	
	// 日志级别
	b.WriteString("level=")
	b.WriteString(entry.Level.String())
	b.WriteByte(' ')
	
	// 消息
	b.WriteString("msg=")
	b.WriteString(f.quoteValue(entry.Message))
	
	// 错误信息
	if entry.Error != "" {
		b.WriteString(" error=")
		b.WriteString(f.quoteValue(entry.Error))
	}
	
	// 调用者信息
	if entry.Caller != nil {
		b.WriteString(" file=")
		b.WriteString(f.quoteValue(fmt.Sprintf("%s:%d", entry.Caller.File, entry.Caller.Line)))
		b.WriteString(" func=")
		b.WriteString(f.quoteValue(entry.Caller.Function))
	}
	
	// 字段
	keys := make([]string, 0, len(entry.Fields))
	for k := range entry.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	for _, key := range keys {
		value := entry.Fields[key]
		b.WriteByte(' ')
		
		// 字段映射
		if f.FieldMap != nil {
			if mappedKey, ok := f.FieldMap[key]; ok {
				key = mappedKey
			}
		}
		
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(f.formatValue(value))
	}
	
	b.WriteByte('\n')
	return b.Bytes(), nil
}

// formatValue 格式化值
func (f *LogfmtFormatter) formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return f.quoteValue(v)
	case error:
		return f.quoteValue(v.Error())
	case time.Time:
		return v.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// quoteValue 引用值
func (f *LogfmtFormatter) quoteValue(s string) string {
	if s == "" {
		return `""`
	}
	
	if strings.ContainsAny(s, " \t\n\r=\"") {
		return fmt.Sprintf(`"%s"`, strings.ReplaceAll(s, `"`, `\"`))
	}
	return s
}

// CustomFormatter 自定义格式化器
type CustomFormatter struct {
	FormatFunc func(*Entry) ([]byte, error)
}

// Format 格式化日志条目
func (f *CustomFormatter) Format(entry *Entry) ([]byte, error) {
	if f.FormatFunc == nil {
		return nil, fmt.Errorf("format function is nil")
	}
	return f.FormatFunc(entry)
}

// NewTextFormatter 创建文本格式化器
func NewTextFormatter() *TextFormatter {
	return &TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		PadLevelText:    true,
	}
}

// NewJSONFormatter 创建JSON格式化器
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
	}
}

// NewLogfmtFormatter 创建Logfmt格式化器
func NewLogfmtFormatter() *LogfmtFormatter {
	return &LogfmtFormatter{
		TimestampFormat: time.RFC3339,
	}
}

// NewCustomFormatter 创建自定义格式化器
func NewCustomFormatter(formatFunc func(*Entry) ([]byte, error)) *CustomFormatter {
	return &CustomFormatter{
		FormatFunc: formatFunc,
	}
}