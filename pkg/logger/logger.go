package logger

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 通过zap和lumberjack实现日志系统，其中zap是日志库，lumberjack是日志切割库

const (
	// DefaultLevel the default log level
	DefaultLevel = zapcore.InfoLevel

	// DefaultTimeLayout the default time layout;
	DefaultTimeLayout = time.RFC3339
)

// Option custom setup config
type Option func(*option)

// option结构体中是可选参数，通过设计模式中的函数式选项模式来实现
type option struct {
	// 日志级别
	level zapcore.Level
	// 日志额外输出的K-V
	fields map[string]string
	// 写日志的writer
	file io.Writer
	// 输出的时间格式
	timeLayout string
	// 是否禁用控制台输出
	disableConsole bool
}

// WithDebugLevel only greater than 'level' will output
func WithDebugLevel() Option {
	return func(opt *option) {
		opt.level = zapcore.DebugLevel
	}
}

// WithInfoLevel only greater than 'level' will output
func WithInfoLevel() Option {
	return func(opt *option) {
		opt.level = zapcore.InfoLevel
	}
}

// WithWarnLevel only greater than 'level' will output
func WithWarnLevel() Option {
	return func(opt *option) {
		opt.level = zapcore.WarnLevel
	}
}

// WithErrorLevel only greater than 'level' will output
func WithErrorLevel() Option {
	return func(opt *option) {
		opt.level = zapcore.ErrorLevel
	}
}

// WithField add some field(s) to log
func WithField(key, value string) Option {
	return func(opt *option) {
		opt.fields[key] = value
	}
}

// WithFileP write log to some file （写文件到单一文件）
func WithFileP(file string) Option {
	dir := filepath.Dir(file)
	// 创建日志目录，已经存在不会报错，不存在则创建
	if err := os.MkdirAll(dir, 0766); err != nil {
		panic(err)
	}
	// 打开这个文件，如果不存在则创建，如果存在则追加
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0766)
	if err != nil {
		panic(err)
	}

	return func(opt *option) {
		// 将f使用zapcore.Lock包装，使其并发安全，然后赋值给opt.file（对于*os.File，使用zap时必须Lock下）
		opt.file = zapcore.Lock(f)
	}
}

// WithFileRotationP write log to some file with rotation （带切割写文件）
func WithFileRotationP(file string) Option {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0766); err != nil {
		panic(err)
	}

	return func(opt *option) {
		// 使用lumberjack库实现日志切割
		opt.file = &lumberjack.Logger{ // concurrent-safed
			Filename:   file, // 文件路径
			MaxSize:    128,  // 单个文件最大尺寸，默认单位 M
			MaxBackups: 300,  // 最多保留 300 个备份
			MaxAge:     30,   // 最大时间，默认单位 day
			LocalTime:  true, // 使用本地时间
			Compress:   true, // 是否压缩 disabled by default
		}
	}
}

// WithTimeLayout custom time format
func WithTimeLayout(timeLayout string) Option {
	return func(opt *option) {
		opt.timeLayout = timeLayout
	}
}

// WithDisableConsole WithEnableConsole write log to os.Stdout or os.Stderr
func WithDisableConsole() Option {
	return func(opt *option) {
		opt.disableConsole = true
	}
}

// NewJSONLogger return a json-encoder zap logger,
func NewJSONLogger(opts ...Option) (*zap.Logger, error) {
	// 初始化option结构体，默认日志级别
	opt := &option{level: DefaultLevel, fields: make(map[string]string)}
	for _, f := range opts {
		f(opt)
	}
	// 如果没有指定layout，使用默认的layout
	timeLayout := DefaultTimeLayout
	if opt.timeLayout != "" {
		timeLayout = opt.timeLayout
	}

	// similar to zap.NewProductionEncoderConfig()
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "time",                        // 自定义输出日志中，时间的key名称
		LevelKey:      "level",                       // 自定义输出日志中，日志级别的key名称
		NameKey:       "logger",                      // 被logger.Named(key)使用，可选字段，可默认
		CallerKey:     "caller",                      // 自定义输出日志中，调用处的key名称
		MessageKey:    "msg",                         // 自定义输出日志中，错误信息的key名称
		StacktraceKey: "stacktrace",                  // use by zap.AddStacktrace; optional; useless
		LineEnding:    zapcore.DefaultLineEnding,     // 换行符
		EncodeLevel:   zapcore.LowercaseLevelEncoder, // 对level字段的编码器（大写、小写等）
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format(timeLayout)) // 自定义时间格式
		},
		EncodeDuration: zapcore.MillisDurationEncoder, // 时间精度，默认毫秒
		EncodeCaller:   zapcore.ShortCallerEncoder,    // 全路径编码器，定义输出日志中的调用函数位置的格式
	}

	jsonEncoder := zapcore.NewJSONEncoder(encoderConfig)

	// 下面两个优先级是为了控台输出时使用的
	// lowPriority usd by info\debug\warn
	lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= opt.level && lvl < zapcore.ErrorLevel
	})

	// highPriority usd by error\panic\fatal
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= opt.level && lvl >= zapcore.ErrorLevel
	})

	// stdout and stderr加锁
	stdout := zapcore.Lock(os.Stdout) // lock for concurrent safe
	stderr := zapcore.Lock(os.Stderr) // lock for concurrent safe

	// 初始化core，file的优先级比控制台高
	core := zapcore.NewTee()

	// 如果允许控制台输出，stdout和stderr分别使用两个不同的优先级，然后使用zapcore.NewTee合并
	if !opt.disableConsole {
		core = zapcore.NewTee(
			// 标准输出使用低优先级
			zapcore.NewCore(jsonEncoder,
				zapcore.NewMultiWriteSyncer(stdout),
				lowPriority,
			),
			// 错误输出使用高优先级
			zapcore.NewCore(jsonEncoder,
				zapcore.NewMultiWriteSyncer(stderr),
				highPriority,
			),
		)
	}
	// 如果指定了文件输出，重新给core赋值
	if opt.file != nil {
		core = zapcore.NewTee(core,
			zapcore.NewCore(jsonEncoder,
				// 使用AddSync添加文件输出
				zapcore.AddSync(opt.file),
				// 如果指定了文件输出，只要级别比定义的高，就写入文件
				zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
					return lvl >= opt.level
				}),
			),
		)
	}
	// 最终创建logger
	logger := zap.New(core,
		zap.AddCaller(),         // 打开caller，可以查看调用函数的文件、行号等信息
		zap.ErrorOutput(stderr), // 设置错误输出，如果不设置，默认输出到stderr
	)

	// 添加自定义字段
	for key, value := range opt.fields {
		logger = logger.WithOptions(zap.Fields(zapcore.Field{Key: key, Type: zapcore.StringType, String: value}))
	}
	return logger, nil
}

// 下面的meta相关的代码，可以在输出日志时，添加额外的key-value，而不是在定义logger时添加

// 这里保证meta实现了Meta接口
var _ Meta = (*meta)(nil)

// Meta key-value
type Meta interface {
	Key() string
	Value() interface{}
	meta() // 这种小写接口方法的方式，可以保证只有包内的代码才能实现该接口
}

type meta struct {
	key   string
	value interface{}
}

func (m *meta) Key() string {
	return m.key
}

func (m *meta) Value() interface{} {
	return m.value
}

func (m *meta) meta() {}

// NewMeta create meta
func NewMeta(key string, value interface{}) Meta {
	return &meta{key: key, value: value}
}

// WrapMeta wrap meta to zap fields
func WrapMeta(err error, metas ...Meta) (fields []zap.Field) {
	capacity := len(metas) + 1 // namespace meta
	if err != nil {
		capacity++
	}

	fields = make([]zap.Field, 0, capacity)
	if err != nil {
		fields = append(fields, zap.Error(err))
	}

	fields = append(fields, zap.Namespace("meta"))
	for _, meta := range metas {
		fields = append(fields, zap.Any(meta.Key(), meta.Value()))
	}

	return
}
