package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.SugaredLogger

func Init(isDevelopment bool, logLevel string) {
	var base *zap.Logger
	var level zapcore.Level

	// Parse log level, default based on environment
	switch logLevel {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	default:
		if isDevelopment {
			level = zapcore.DebugLevel
		} else {
			level = zapcore.InfoLevel
		}
	}

	var config zap.Config
	if isDevelopment {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}
	config.Level = zap.NewAtomicLevelAt(level)

	base, _ = config.Build()
	Log = base.Sugar()
}

func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}
