package logger

import (
	"go.uber.org/zap"

	"github.com/DIN-center/din-caddy-plugins/lib/utils"
)

type LoggerClient struct {
	Logger *zap.Logger
}

func NewLoggerClient(logger *zap.Logger, env utils.Environment) *LoggerClient {
	// Add context to ensure all logs include ENV and MachineID fields.
	loggerWithContext := logger.With(zap.String("machine_id", utils.GetMachineId()),
		zap.String("environment", string(env)))
	return &LoggerClient{
		Logger: loggerWithContext,
	}
}

func (l *LoggerClient) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

func (l *LoggerClient) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

func (l *LoggerClient) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
}

func (l *LoggerClient) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

func (l *LoggerClient) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, fields...)
}
