package logger

import (
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"go.uber.org/zap"
)

type LoggerClient struct {
	Logger    *zap.Logger
	machineId string
	env       utils.Environment
}

func NewLoggerClient(logger *zap.Logger, env utils.Environment) *LoggerClient {
	return &LoggerClient{
		Logger:    logger,
		machineId: utils.GetMachineId(),
		env:       env,
	}
}

// addCommonFields adds standard fields to all log entries
func (l *LoggerClient) addCommonFields(fields ...zap.Field) []zap.Field {
	return append(fields,
		zap.String("machine_id", l.machineId),
		zap.String("environment", string(l.env)))
}

func (l *LoggerClient) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, l.addCommonFields(fields...)...)
}

func (l *LoggerClient) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, l.addCommonFields(fields...)...)
}

func (l *LoggerClient) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, l.addCommonFields(fields...)...)
}

func (l *LoggerClient) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, l.addCommonFields(fields...)...)
}

func (l *LoggerClient) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, l.addCommonFields(fields...)...)
}
