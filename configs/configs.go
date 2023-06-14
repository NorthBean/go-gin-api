package configs

import (
	"bytes"
	_ "embed"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/xinliangnote/go-gin-api/pkg/env"
	"github.com/xinliangnote/go-gin-api/pkg/file"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var config = new(Config)

// Config 配置文件结构体，参考fat_configs.toml
type Config struct {
	MySQL struct {
		Read struct {
			Addr string `toml:"addr"`
			User string `toml:"user"`
			Pass string `toml:"pass"`
			Name string `toml:"name"`
		} `toml:"read"`
		Write struct {
			Addr string `toml:"addr"`
			User string `toml:"user"`
			Pass string `toml:"pass"`
			Name string `toml:"name"`
		} `toml:"write"`
		Base struct {
			MaxOpenConn     int           `toml:"maxOpenConn"`
			MaxIdleConn     int           `toml:"maxIdleConn"`
			ConnMaxLifeTime time.Duration `toml:"connMaxLifeTime"`
		} `toml:"base"`
	} `toml:"mysql"`

	Redis struct {
		Addr         string `toml:"addr"`
		Pass         string `toml:"pass"`
		Db           int    `toml:"db"`
		MaxRetries   int    `toml:"maxRetries"`
		PoolSize     int    `toml:"poolSize"`
		MinIdleConns int    `toml:"minIdleConns"`
	} `toml:"redis"`

	Mail struct {
		Host string `toml:"host"`
		Port int    `toml:"port"`
		User string `toml:"user"`
		Pass string `toml:"pass"`
		To   string `toml:"to"`
	} `toml:"mail"`

	HashIds struct {
		Secret string `toml:"secret"`
		Length int    `toml:"length"`
	} `toml:"hashids"`

	Language struct {
		Local string `toml:"local"`
	} `toml:"language"`
}

// 这里使用了go1.16的embed特性，将配置文件内容嵌入到变量中
var (
	//go:embed dev_configs.toml
	devConfigs []byte

	//go:embed fat_configs.toml
	fatConfigs []byte

	//go:embed uat_configs.toml
	uatConfigs []byte

	//go:embed pro_configs.toml
	proConfigs []byte
)

func init() {
	var r io.Reader
	// 根据参数中env的设置选择配置文件，因为env在init之前已经被初始化了，所以这里可以直接使用
	switch env.Active().Value() {
	case "dev":
		r = bytes.NewReader(devConfigs)
	case "fat":
		r = bytes.NewReader(fatConfigs)
	case "uat":
		r = bytes.NewReader(uatConfigs)
	case "pro":
		r = bytes.NewReader(proConfigs)
	default:
		r = bytes.NewReader(fatConfigs)
	}
	// 使用viper库处理配置文件，支持配置文件的远程加载和动态更新
	// 设置配置文件格式为toml
	viper.SetConfigType("toml")
	// 读取配置文件
	if err := viper.ReadConfig(r); err != nil {
		panic(err)
	}
	// 将配置文件内容反序列化到config结构体中
	if err := viper.Unmarshal(config); err != nil {
		panic(err)
	}
	// 关联具体的配置文件，便于后面监听配置文件变化
	viper.SetConfigName(env.Active().Value() + "_configs")
	viper.AddConfigPath("./configs")

	configFile := "./configs/" + env.Active().Value() + "_configs.toml"
	_, ok := file.IsExists(configFile)
	if !ok {
		if err := os.MkdirAll(filepath.Dir(configFile), 0766); err != nil {
			panic(err)
		}

		f, err := os.Create(configFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()

		if err := viper.WriteConfig(); err != nil {
			panic(err)
		}
	}
	// 监听配置文件变化，如果变化，将新的内容反序列化到config结构体中
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		if err := viper.Unmarshal(config); err != nil {
			panic(err)
		}
	})
}

func Get() Config {
	return *config
}
