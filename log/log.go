// Copyright 2022 The kubegems.io Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 背景： https://github.com/go-logr/logr#background
// 总体上有几条：

// 1. 日志需要结构化，在云原生的环境下，结构化的日志相比于 format 日志更机器可读，可统计，可搜索，信息要素完善。
// 2. 日志最终仅分为错误和没有错误两类。对应 .Info .Error 函数。（警告级别的日志没有人会关心，所以根本没用）see: https://dave.cheney.net/2015/11/05/lets-talk-about-logging
// 3. 对于非错误日志,除了用debug trace warn 等区别等级 ，还可以用更灵活的 .V() 设置等级，可以自行判断根据日志重要性设置。
// 对于 caller，如果logger使用的正确，是不需要caller的 使用 .WithName() 可以手动区分logger所在模块，且使用caller stacktrace 会增加额外的开销。

var (
	NewContext           = logr.NewContext
	FromContextOrDiscard = logr.FromContextOrDiscard
)

var (
	Info  = Logger.Info
	Error = Logger.Error
	V     = Logger.V
	Warn  = Logger.V(1)
	Debug = Logger.V(2)
	Trace = Logger.V(3)
)

const TimeFormat = "2006-01-02 15:04:05.999"

var zapLogger, Logger = MustNewLogger()

var AtomicLevel = zap.NewAtomicLevelAt(zap.InfoLevel) // 通过更改 level 可一更改runtime logger的level

func MustNewLogger() (*zap.Logger, logr.Logger) {
	config := zap.Config{
		Level:    AtomicLevel,
		Encoding: "console", // console json
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "ts",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.TimeEncoderOfLayout(TimeFormat),
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:       []string{"stderr"},
		ErrorOutputPaths:  []string{"stderr"},
		DisableStacktrace: true,
	}

	// level from env
	_ = config.Level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL")))

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	return logger, zapr.NewLogger(logger)
}
