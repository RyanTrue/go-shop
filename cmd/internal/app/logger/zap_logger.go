package logger

import "go.uber.org/zap"

func InitLogger(level string) (*zap.SugaredLogger, error) {

	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return nil, err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl

	zl, err := cfg.Build()
	if err != nil {
		return nil, err
	}

	return zl.Sugar(), nil

}
