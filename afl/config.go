package afl

import (
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/go-viper/encoding/ini"
	"github.com/spf13/viper"
)

var v *viper.Viper

func newv() *viper.Viper {
	codecRegistry := viper.NewCodecRegistry()
	codecRegistry.RegisterCodec("ini", ini.Codec{})
	v := viper.NewWithOptions(viper.WithCodecRegistry(codecRegistry))
	v.SetConfigFile("config.ini")
	v.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
	})
	v.WatchConfig()
	err := v.ReadInConfig()
	if err != nil {
		panic(err)
	}
	return v
}

func GetDsnInfo() (user, password, addr string) {
	return v.GetString("mysql.user"), v.GetString("mysql.password"), v.GetString("mysql.addr")
}
