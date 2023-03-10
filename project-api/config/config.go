package config

import (
	"github.com/axzed/project-common/logs"
	"github.com/spf13/viper"
	"log"
	"os"
)

var AppConf = NewConfig()

type Config struct {
	viper      *viper.Viper
	SC         *ServerConfig
	GC         *GrpcConfig
	EtcdConfig *EtcdConfig
}

// GrpcConfig grpc配置
type GrpcConfig struct {
	Name    string `mapstructure:"name"`
	Addr    string `mapstructure:"addr"`
	Version string `mapstructure:"version"`
	Weight  int64  `mapstructure:"weight"`
}

// EtcdConfig etcd配置
type EtcdConfig struct {
	Addrs []string `mapstructure:"addrs"`
}

// NewConfig 初始化配置
func NewConfig() *Config {
	v := viper.New()
	conf := &Config{viper: v}
	workDir, _ := os.Getwd()
	conf.viper.SetConfigName("app")
	conf.viper.SetConfigType("yaml")
	conf.viper.AddConfigPath(workDir + "/config")

	err := conf.viper.ReadInConfig()
	if err != nil {
		log.Fatalln(err)
		return nil
	}
	conf.InitServerConfig()
	conf.InitZapLog()
	conf.InitEtcdConfig()

	return conf
}

type ServerConfig struct {
	Name string `mapstructure:"name"`
	Addr string `mapstructure:"addr"`
}

// InitServerConfig 初始化服务配置
func (c *Config) InitServerConfig() {
	sc := &ServerConfig{}
	sc.Name = c.viper.GetString("server.name")
	sc.Addr = c.viper.GetString("server.addr")
	c.SC = sc
}

// InitZapLog 初始化zap日志
func (c *Config) InitZapLog() {
	//从配置中读取日志配置，初始化日志
	lc := &logs.LogConfig{
		DebugFileName: c.viper.GetString("zap.debugFileName"),
		InfoFileName:  c.viper.GetString("zap.infoFileName"),
		WarnFileName:  c.viper.GetString("zap.warnFileName"),
		MaxSize:       c.viper.GetInt("maxSize"),
		MaxAge:        c.viper.GetInt("maxAge"),
		MaxBackups:    c.viper.GetInt("maxBackups"),
	}
	err := logs.InitLogger(lc)
	if err != nil {
		log.Fatalln(err)
	}
}

// InitGrpcConfig 初始化grpc配置
func (c *Config) InitGrpcConfig() {
	gc := &GrpcConfig{}
	gc.Name = c.viper.GetString("grpc.name")
	gc.Addr = c.viper.GetString("grpc.addr")
	gc.Version = c.viper.GetString("grpc.version")
	gc.Weight = c.viper.GetInt64("grpc.weight")
	c.GC = gc
}

// InitEtcdConfig 初始化etcd配置
func (c *Config) InitEtcdConfig() {
	ec := &EtcdConfig{}
	var addrs []string
	err := c.viper.UnmarshalKey("etcd.addrs", &addrs)
	if err != nil {
		log.Fatalln(err)
	}
	ec.Addrs = addrs
	c.EtcdConfig = ec
}
