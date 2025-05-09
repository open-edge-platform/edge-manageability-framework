package internal

import (
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func parseLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zap.DebugLevel
	case "info":
		return zap.InfoLevel
	case "warn":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel // Default to info level
	}
}

func InitLogger(logLevel string, logDir string) error {
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		err := os.MkdirAll(logDir, os.ModePerm)
		if err != nil {
			return err
		}
	}
	logFile := filepath.Join(logDir, "orch-installer.log")
	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.Level.SetLevel(parseLogLevel(logLevel))
	loggerConfig.OutputPaths = []string{"stdout", logFile}
	loggerRoot, err := loggerConfig.Build()
	if err != nil {
		return err
	}
	zap.ReplaceGlobals(loggerRoot)
	zap.S().Infof("Log level set to %s", logLevel)

	return nil
}

func Logger() *zap.SugaredLogger {
	return zap.S()
}

func FileLogWriter(logFile string) (io.Writer, error) {
	return os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}
