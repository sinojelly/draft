package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	InfoLog  *log.Logger
	ErrorLog *log.Logger
	sinks    []func(string, string)
	sinkMu   sync.RWMutex

	levelValue   int
	currentLevel string
)

const (
	LevelDebug = "DEBUG"
	LevelInfo  = "INFO"
	LevelWarn  = "WARN"
	LevelError = "ERROR"
)

var levelMap = map[string]int{
	LevelDebug: 0,
	LevelInfo:  1,
	LevelWarn:  2,
	LevelError: 3,
}

// LogSink is a function that receives log messages
type LogSink func(string, string)

// throttleCache counts consecutive errors of the same signature
type throttleCache struct {
	Count     int
	FirstSeen time.Time
	LastSeen  time.Time
}

var (
	throttlerMap sync.Map // [string]*throttleCache
)

func InitLogger(logDir, level string) {
	currentLevel = strings.ToUpper(level)
	if v, ok := levelMap[currentLevel]; ok {
		levelValue = v
	} else {
		levelValue = 1 // Default to INFO
		currentLevel = LevelInfo
	}

	err := os.MkdirAll(logDir, 0755)
	if err != nil {
		log.Fatal("Failed to create log directory:", err)
	}

	infoWriter := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "info.log"),
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   // days
		Compress:   true, // compress rotated files
	}

	errorWriter := &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "error.log"),
		MaxSize:    10, // megabytes
		MaxBackups: 5,
		MaxAge:     28,
		Compress:   true,
	}

	InfoLog = log.New(infoWriter, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLog = log.New(errorWriter, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	LogInfo("Logger initialized at level: %s", currentLevel)

	// Start throttler cleaner
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			flushThrottledErrors()
		}
	}()
}

// flushThrottledErrors prints aggregated errors to the file
func flushThrottledErrors() {
	throttlerMap.Range(func(key, value interface{}) bool {
		signature := key.(string)
		cache := value.(*throttleCache)

		if cache.Count > 0 {
			msg := fmt.Sprintf("Throttled Error [%s] occurred %d times between %v and %v",
				signature, cache.Count, cache.FirstSeen.Format(time.RFC3339), cache.LastSeen.Format(time.RFC3339))
			ErrorLog.Println(msg)

			// Reset
			throttlerMap.Delete(key)
		}
		return true
	})
}

// LogError throttles repeating errors by signature {AccountID}:{ErrType}
func LogError(signature string, err error) {
	now := time.Now()

	val, loaded := throttlerMap.LoadOrStore(signature, &throttleCache{
		Count:     1,
		FirstSeen: now,
		LastSeen:  now,
	})

	if loaded {
		cache := val.(*throttleCache)
		cache.Count++
		cache.LastSeen = now
		throttlerMap.Store(signature, cache)
	} else {
		// Log the first occurrence immediately
		ErrorLog.Printf("[%s] First occurrence: %v", signature, err)
	}
}

// LogDebug writes a debug log if level permits
func LogDebug(format string, v ...interface{}) {
	if levelValue > 0 {
		return
	}
	msg := fmt.Sprintf(format, v...)
	if InfoLog != nil {
		InfoLog.Println("DEBUG: " + msg)
	}
	broadcastToSinks("DEBUG", msg)
}

// LogInfo writes a standard info log
func LogInfo(format string, v ...interface{}) {
	if levelValue > 1 {
		return
	}
	msg := fmt.Sprintf(format, v...)
	if InfoLog != nil {
		InfoLog.Println(msg)
	} else {
		log.Printf("[INFO] %s", msg)
	}
	broadcastToSinks("INFO", msg)
}

// LogWarn writes a warning log
func LogWarn(format string, v ...interface{}) {
	if levelValue > 2 {
		return
	}
	msg := fmt.Sprintf(format, v...)
	if InfoLog != nil {
		InfoLog.Println("WARN: " + msg)
	}
	broadcastToSinks("WARN", msg)
}

// LogErrorImmediate (unthrottled) for critical system errors
func LogErrorImmediate(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if ErrorLog != nil {
		ErrorLog.Println(msg)
	} else {
		log.Printf("[ERROR] %s", msg)
	}
	broadcastToSinks("ERROR", msg)
}

// RegisterSink adds a custom log handler (e.g., for Telemetry)
func RegisterSink(sink func(string, string)) {
	sinkMu.Lock()
	defer sinkMu.Unlock()
	sinks = append(sinks, sink)
}

func broadcastToSinks(level, msg string) {
	sinkMu.RLock()
	defer sinkMu.RUnlock()
	for _, s := range sinks {
		s(level, msg)
	}
}

func SetLevel(levelStr string) {
	levelStr = strings.ToUpper(levelStr)
	if v, ok := levelMap[levelStr]; ok {
		levelValue = v
		currentLevel = levelStr
		LogInfo("[Logger] 动态热重载：日志级别已切换为 %s", levelStr)
	} else {
		LogWarn("[Logger] 动态热重载：日志级别 %s 不存在，保持不变", levelStr)
	}
}
