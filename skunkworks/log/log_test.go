// Unfinished test
package log

import (
	"testing"
)

func TestDefaultCfg(t *testing.T) {
	cfg := LogConfig{
		Level:      "info",
		Files:      []string{"./test.log"},
		Underlying: "zap",
	}
	cfg.ZapConfig.MaxSize = 100
	cfg.ZapConfig.MaxBackups = 0
	cfg.ZapConfig.MaxAge = 0
	cfg.ZapConfig.LocalTime = true
	cfg.ZapConfig.Compress = true
	cfg.ZapConfig.JsonFormat = false
	cfg.ZapConfig.BtEnabled = true
	cfg.ZapConfig.BtLevel = "error"
	Setup(&cfg)

	Debug("debug msg", 33, "33")
	Debugf("debug msg %d %s", 33, "33")
	Debugln("debug msg", 33, "33")
	Debugw("debug msg", KeysAndValues{"33", 44, "55", 66})
	Print("print msg", 33, "33")
	Printf("print msg %d %s", 33, "33")
	Println("print msg", 33, "33")
	Printw("print msg", KeysAndValues{"33", 44, "55", 66})
	Info("info msg", 33, "33")
	Infof("info msg %d %s", 33, "33")
	Infoln("info msg", 33, "33")
	Infow("info msg", KeysAndValues{"33", 44, "55", 66})
	Warn("warn msg", 33, "33")
	Warnf("warn msg %d %s", 33, "33")
	Warnln("warn msg", 33, "33")
	Warnw("warn msg", KeysAndValues{"33", 44, "55", 66})
	Error("error msg", 33, "33")
	Errorf("error msg %d %s", 33, "33")
	Errorln("error msg", 33, "33")
	Errorw("error msg", KeysAndValues{"33", 44, "55", 66})
	DPanic("dpanic msg", 33, "33")
	DPanicf("dpanic msg %d %s", 33, "33")
	DPanicln("dpanic msg", 33, "33")
	DPanicw("dpanic msg", KeysAndValues{"33", 44, "55", 66})
	Panic("panic msg", 33, "33")
	Panicf("panic msg %d %s", 33, "33")
	Panicln("panic msg", 33, "33")
	Panicw("panic msg", KeysAndValues{"33", 44, "55", 66})
	Fatal("fatal msg", 33, "33")
	Fatalf("fatal msg %d %s", 33, "33")
	Fatalln("fatal msg", 33, "33")
	Fatalw("fatal msg", KeysAndValues{"33", 44, "55", 66})
}
