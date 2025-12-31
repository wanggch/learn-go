package obs

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

type Logger struct {
	service string
	logger  *log.Logger
}

type AppError struct {
	Op    string
	Kind  string
	Err   error
	Trace string
}

func NewLogger(service string) *Logger {
	return &Logger{
		service: service,
		logger:  log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *Logger) Info(msg string, fields ...Field) {
	l.emit("INFO", msg, fields...)
}

func (l *Logger) Error(err error, fields ...Field) {
	l.emit("ERROR", "error", append(fields, Field{"err", err.Error()})...)
}

func (l *Logger) ErrorWithTrace(err error, fields ...Field) {
	if app, ok := AsAppError(err); ok && app.Trace != "" {
		fields = append(fields, Field{Key: "err_trace", Value: app.Trace})
	}
	l.Error(err, fields...)
}

func (l *Logger) emit(level, msg string, fields ...Field) {
	parts := []string{
		"level=" + level,
		"service=" + l.service,
		"msg=" + msg,
	}
	for _, f := range fields {
		parts = append(parts, f.String())
	}
	l.logger.Println(strings.Join(parts, " "))
}

type Field struct {
	Key   string
	Value string
}

func (f Field) String() string {
	return f.Key + "=" + f.Value
}

func Str(key, val string) Field {
	return Field{Key: key, Value: val}
}

func Int(key string, val int) Field {
	return Field{Key: key, Value: fmt.Sprintf("%d", val)}
}

func Duration(key string, val time.Duration) Field {
	return Field{Key: key, Value: val.String()}
}

func Wrap(op, kind, trace string, err error) error {
	if err == nil {
		return nil
	}
	return AppError{Op: op, Kind: kind, Trace: trace, Err: err}
}

func (e AppError) Error() string {
	return fmt.Sprintf("%s: %s: %v", e.Op, e.Kind, e.Err)
}

func (e AppError) Unwrap() error {
	return e.Err
}

func IsKind(err error, kind string) bool {
	var app AppError
	if errors.As(err, &app) {
		return app.Kind == kind
	}
	return false
}

func AsAppError(err error) (AppError, bool) {
	var app AppError
	if errors.As(err, &app) {
		return app, true
	}
	return AppError{}, false
}

func TraceID() string {
	return fmt.Sprintf("trace-%d", time.Now().UnixNano())
}
