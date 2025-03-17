package logger

import (
	"github.com/DIN-center/din-caddy-plugins/lib/utils"
	"go.uber.org/zap"
)

type LoggerClient struct {
	Logger    *zap.Logger
	machineId string
}

func NewLoggerClient(logger *zap.Logger) *LoggerClient {
	machineId := utils.GetMachineId()
	return &LoggerClient{
		Logger:    logger,
		machineId: machineId,
	}
}

func (l *LoggerClient) Info(msg string, fields ...zap.Field) {
	allFields := append(fields, zap.String("machine_id", l.machineId))
	l.Logger.Info(msg, allFields...)
}

func (l *LoggerClient) Debug(msg string, fields ...zap.Field) {
	allFields := append(fields, zap.String("machine_id", l.machineId))
	l.Logger.Debug(msg, allFields...)
}

func (l *LoggerClient) Error(msg string, fields ...zap.Field) {
	allFields := append(fields, zap.String("machine_id", l.machineId))
	l.Logger.Error(msg, allFields...)
}

func (l *LoggerClient) Warn(msg string, fields ...zap.Field) {
	allFields := append(fields, zap.String("machine_id", l.machineId))
	l.Logger.Warn(msg, allFields...)
}

func (l *LoggerClient) Fatal(msg string, fields ...zap.Field) {
	allFields := append(fields, zap.String("machine_id", l.machineId))
	l.Logger.Fatal(msg, allFields...)
}
