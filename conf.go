package main

import (
	"github.com/spf13/viper"
)

type FetchConf struct {
	Client struct {
		Interval     int
		Method       string
		SelectOrigin string
		CustomUrl    string
	}
	Server struct {
		Interval int
		Port     int
	}
}

func (f *FetchConf) Storage() {
	viper.Set("client.interval", f.Client.Interval)
	viper.Set("client.method", f.Client.Method)
	viper.Set("client.selectorigin", f.Client.SelectOrigin)
	viper.Set("client.customurl", f.Client.CustomUrl)
//	viper.Set("server.interval", f.Server.Interval)
//	viper.Set("server.port", f.Server.Port)
	if err := viper.WriteConfigAs("conf.yaml"); err != nil {
		_fileLog.Print("持久化配置信息失败：" + err.Error())
	}
}

func LoadFetchConf() *FetchConf {
	viper.AddConfigPath(AppExecDir())
	viper.SetConfigName("conf")
	viper.SetConfigType("yaml")
	viper.SetDefault("client.interval", 60)
	viper.SetDefault("client.method", "指定的hosts源")
	viper.SetDefault("client.selectorigin", "MiniYun-Hosts")
//	viper.SetDefault("server.interval", 60)
//	viper.SetDefault("server.port", 9898)
	var fileNotExits bool
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			fileNotExits = true
		} else {
			_fileLog.Print("加载配置文件错误： " + err.Error())
		}
	}
	res := FetchConf{}
	if err := viper.Unmarshal(&res); err != nil {
		_fileLog.Print("配置文件解析失败")
	}
	if fileNotExits {
		res.Storage()
	}
	return &res
}
