package log

import (
	"fmt"
	"os"

	"github.com/finogeeks/ligase/skunkworks/zap"
	"github.com/finogeeks/ligase/skunkworks/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type ZapLogger struct {
	logger *zap.SugaredLogger
}

func getLevel(level string) zapcore.Level {
	var l zapcore.Level
	switch level {
	case "debug":
		l = zapcore.DebugLevel
	case "info":
		l = zapcore.InfoLevel
	case "warn":
		l = zapcore.WarnLevel
	case "error":
		l = zapcore.ErrorLevel
	case "dpanic":
		l = zapcore.DPanicLevel
	case "panic":
		l = zapcore.PanicLevel
	case "fatal":
		l = zapcore.FatalLevel
	}
	return l
}

func getSeparator(separator string) byte {
	var sep byte
	switch separator {
	case "space":
		sep = ' '
	case "tab":
		sep = '\t'
	default:
		sep = ' '
	}
	return sep
}

func newZapLogger(cfg *LogConfig) Logger {
	zapLogger := &ZapLogger{
		logger: zap.S(),
	}
	syncers := make([]zapcore.WriteSyncer, 0, len(cfg.Files)+1)
	if cfg.WriteToStdout {
		syncers = append(syncers, zapcore.AddSync(os.Stdout))
	}

	for _, v := range cfg.Files {
		syncers = append(syncers, zapcore.AddSync(&lumberjack.Logger{
			Filename:   v,
			MaxSize:    cfg.ZapConfig.MaxSize,
			MaxBackups: cfg.ZapConfig.MaxBackups,
			MaxAge:     cfg.ZapConfig.MaxAge,
			LocalTime:  cfg.ZapConfig.LocalTime,
			Compress:   cfg.ZapConfig.Compress,
		}))
	}

	encCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		TimeKey:        "time",
		CallerKey:      "caller",
		StacktraceKey:  "bt",
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
		FieldSeparator: getSeparator(cfg.ZapConfig.FieldSeparator),
	}

	var encoder zapcore.Encoder
	if cfg.ZapConfig.JsonFormat {
		encoder = zapcore.NewJSONEncoder(encCfg)
	} else {
		encoder = zapcore.NewConsoleEncoder(encCfg)
	}
	logger := zap.New(zapcore.NewCore(
		encoder,
		zapcore.NewMultiWriteSyncer(syncers...),
		getLevel(cfg.Level),
	))

	if cfg.ZapConfig.BtEnabled {
		logger = logger.WithOptions(zap.AddStacktrace(getLevel(cfg.ZapConfig.BtLevel)))
	}
	zap.ReplaceGlobals(logger)
	zapLogger.logger = zap.S()
	return zapLogger
}

func (l *ZapLogger) Debug(args ...interface{}) {
	l.logger.Debug(args...)
}

func (l *ZapLogger) Debugf(template string, args ...interface{}) {
	l.logger.Debugf(template, args...)
}

func (l *ZapLogger) Debugln(args ...interface{}) {
	argsWithBlank := fmt.Sprintln(args...)
	Debug(argsWithBlank[:len(argsWithBlank)-1])
}

func (l *ZapLogger) Debugw(msg string, kv KeysAndValues) {
	l.logger.Debugw(msg, kv...)
}

func (l *ZapLogger) Info(args ...interface{}) {
	l.logger.Info(args...)
}

func (l *ZapLogger) Infof(template string, args ...interface{}) {
	l.logger.Infof(template, args...)
}

func (l *ZapLogger) Infoln(args ...interface{}) {
	argsWithBlank := fmt.Sprintln(args...)
	Info(argsWithBlank[:len(argsWithBlank)-1])
}

func (l *ZapLogger) Infow(msg string, kv KeysAndValues) {
	l.logger.Infow(msg, kv...)
}

func (l *ZapLogger) Warn(args ...interface{}) {
	l.logger.Warn(args...)
}

func (l *ZapLogger) Warnf(template string, args ...interface{}) {
	l.logger.Warnf(template, args...)
}

func (l *ZapLogger) Warnln(args ...interface{}) {
	argsWithBlank := fmt.Sprintln(args...)
	Warn(argsWithBlank[:len(argsWithBlank)-1])
}

func (l *ZapLogger) Warnw(msg string, kv KeysAndValues) {
	l.logger.Warnw(msg, kv...)
}

func (l *ZapLogger) Error(args ...interface{}) {
	l.logger.Error(args...)
}

func (l *ZapLogger) Errorf(template string, args ...interface{}) {
	l.logger.Errorf(template, args...)
}

func (l *ZapLogger) Errorln(args ...interface{}) {
	argsWithBlank := fmt.Sprintln(args...)
	Error(argsWithBlank[:len(argsWithBlank)-1])
}

func (l *ZapLogger) Errorw(msg string, kv KeysAndValues) {
	l.logger.Errorw(msg, kv...)
}

func (l *ZapLogger) DPanic(args ...interface{}) {
	l.logger.DPanic(args...)
}

func (l *ZapLogger) DPanicf(template string, args ...interface{}) {
	l.logger.DPanicf(template, args...)
}

func (l *ZapLogger) DPanicln(args ...interface{}) {
	argsWithBlank := fmt.Sprintln(args...)
	DPanic(argsWithBlank[:len(argsWithBlank)-1])
}

func (l *ZapLogger) DPanicw(msg string, kv KeysAndValues) {
	l.logger.DPanicw(msg, kv...)
}

func (l *ZapLogger) Panic(args ...interface{}) {
	l.logger.Panic(args...)
}

func (l *ZapLogger) Panicf(template string, args ...interface{}) {
	l.logger.Panicf(template, args...)
}

func (l *ZapLogger) Panicln(args ...interface{}) {
	argsWithBlank := fmt.Sprintln(args...)
	Panic(argsWithBlank[:len(argsWithBlank)-1])
}

func (l *ZapLogger) Panicw(msg string, kv KeysAndValues) {
	l.logger.Panicw(msg, kv...)
}

func (l *ZapLogger) Fatal(args ...interface{}) {
	l.logger.Fatal(args...)
}

func (l *ZapLogger) Fatalf(template string, args ...interface{}) {
	l.logger.Fatalf(template, args...)
}

func (l *ZapLogger) Fatalln(args ...interface{}) {
	argsWithBlank := fmt.Sprintln(args...)
	Fatal(argsWithBlank[:len(argsWithBlank)-1])
}

func (l *ZapLogger) Fatalw(msg string, kv KeysAndValues) {
	l.logger.Fatalw(msg, kv...)
}
