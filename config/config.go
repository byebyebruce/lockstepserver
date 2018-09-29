package config

import (
	"github.com/bailu1901/lockstepserver/util"
)

var (
	Cfg = Config{}
)

type Config struct {
	OutAddress    string
	InAddress     string
	EtcdEndPionts string `xml:"etcd_endpoints"`
	EtcdKey       string `xml:"etcd_key"`
	EtcdTTL       int64  `xml:"etcd_ttl"`
	MaxRoom       int    `xml:"max_room"`
}

func LoadConfig(file string) error {
	if err := util.LoadConfig(file, &Cfg); nil != err {
		return err
	}
	return nil
}
