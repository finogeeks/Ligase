package log

import (
	deflog "log"
)

type KeysAndValues []interface{}

type LogConfig struct {
	Level         string
	Files         []string
	Underlying    string
	WriteToStdout bool
	ZapConfig     struct {
		MaxSize        int
		MaxBackups     int
		MaxAge         int
		LocalTime      bool
		Compress       bool
		JsonFormat     bool
		BtEnabled      bool
		BtLevel        string
		FieldSeparator string
	}
}

var logger Logger

func Setup(cfg *LogConfig) {
	switch cfg.Underlying {
	case "zap":
		logger = newZapLogger(cfg)
	default:
		logger = newZapLogger(cfg)
	}
}

func Print(args ...interface{}) {
	Info(args...)
}

func Printf(template string, args ...interface{}) {
	Infof(template, args...)
}

func Println(args ...interface{}) {
	Infoln(args...)
}

func Printw(msg string, kv KeysAndValues) {
	if logger != nil {
		logger.Infow(msg, kv)
	} else {
		deflog.Print(msg)
	}
}

func Debug(args ...interface{}) {
	if logger != nil {
		logger.Debug(args...)
	} else {
		deflog.Print(args...)
	}
}

func Debugf(template string, args ...interface{}) {
	if logger != nil {
		logger.Debugf(template, args...)
	} else {
		deflog.Printf(template, args...)
	}
}

func Debugln(args ...interface{}) {
	if logger != nil {
		logger.Debugln(args...)
	} else {
		deflog.Println(args...)
	}
}

func Debugw(msg string, kv KeysAndValues) {
	if logger != nil {
		logger.Debugw(msg, kv)
	} else {
		deflog.Print(msg, kv)
	}
}

func Info(args ...interface{}) {
	if logger != nil {
		logger.Info(args...)
	} else {
		deflog.Print(args...)
	}
}

func Infof(template string, args ...interface{}) {
	if logger != nil {
		logger.Infof(template, args...)
	} else {
		deflog.Printf(template, args...)
	}
}

func Infoln(args ...interface{}) {
	if logger != nil {
		logger.Infoln(args...)
	} else {
		deflog.Println(args...)
	}
}

func Infow(msg string, kv KeysAndValues) {
	if logger != nil {
		logger.Infow(msg, kv)
	} else {
		deflog.Print(msg, kv)
	}
}

func Warn(args ...interface{}) {
	if logger != nil {
		logger.Warn(args...)
	} else {
		deflog.Print(args...)
	}
}

func Warnf(template string, args ...interface{}) {
	if logger != nil {
		logger.Warnf(template, args...)
	} else {
		deflog.Printf(template, args...)
	}
}

func Warnln(args ...interface{}) {
	if logger != nil {
		logger.Warnln(args...)
	} else {
		deflog.Println(args...)
	}
}

func Warnw(msg string, kv KeysAndValues) {
	if logger != nil {
		logger.Warnw(msg, kv)
	} else {
		deflog.Print(msg, kv)
	}
}

func Error(args ...interface{}) {
	if logger != nil {
		logger.Error(args...)
	} else {
		deflog.Print(args...)
	}
}

func Errorf(template string, args ...interface{}) {
	if logger != nil {
		logger.Errorf(template, args...)
	} else {
		deflog.Printf(template, args...)
	}
}

func Errorln(args ...interface{}) {
	if logger != nil {
		logger.Errorln(args...)
	} else {
		deflog.Println(args...)
	}
}

func Errorw(msg string, kv KeysAndValues) {
	if logger != nil {
		logger.Errorw(msg, kv)
	} else {
		deflog.Print(msg, kv)
	}
}

func DPanic(args ...interface{}) {
	if logger != nil {
		logger.DPanic(args...)
	} else {
		deflog.Panic(args...)
	}
}

func DPanicf(template string, args ...interface{}) {
	if logger != nil {
		logger.DPanicf(template, args...)
	} else {
		deflog.Panicf(template, args...)
	}
}

func DPanicln(args ...interface{}) {
	if logger != nil {
		logger.DPanicln(args...)
	} else {
		deflog.Panicln(args...)
	}
}

func DPanicw(msg string, kv KeysAndValues) {
	if logger != nil {
		logger.DPanicw(msg, kv)
	} else {
		deflog.Panic(msg, kv)
	}
}

func Panic(args ...interface{}) {
	if logger != nil {
		logger.Panic(args...)
	} else {
		deflog.Panic(args...)
	}
}

func Panicf(template string, args ...interface{}) {
	if logger != nil {
		logger.Panicf(template, args...)
	} else {
		deflog.Panicf(template, args...)
	}
}

func Panicln(args ...interface{}) {
	if logger != nil {
		logger.Panicln(args...)
	} else {
		deflog.Panicln(args...)
	}
}

func Panicw(msg string, kv KeysAndValues) {
	if logger != nil {
		logger.Panicw(msg, kv)
	} else {
		deflog.Panic(msg, kv)
	}
}

func Fatal(args ...interface{}) {
	if logger != nil {
		logger.Fatal(args...)
	} else {
		deflog.Fatal(args...)
	}
}

func Fatalf(template string, args ...interface{}) {
	if logger != nil {
		logger.Fatalf(template, args...)
	} else {
		deflog.Fatalf(template, args...)
	}
}

func Fatalln(args ...interface{}) {
	if logger != nil {
		logger.Fatalln(args...)
	} else {
		deflog.Fatalln(args...)
	}
}

func Fatalw(msg string, kv KeysAndValues) {
	if logger != nil {
		logger.Fatalw(msg, kv)
	} else {
		deflog.Fatal(msg, kv)
	}
}
