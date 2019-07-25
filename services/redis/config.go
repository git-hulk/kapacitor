package redis

import "errors"

type Config struct {
	Enabled bool `toml:"enabled" override:"enabled"`
	// The address of redis server, e.g. 127.0.0.1:6379
	Addr string `toml:"addr" override:"address"`
	// The auth passwd of redis
	Password string `toml:"password" override:"auth"`
	// The name of alert queue
	Queue string `toml:"queue" override:"queu"`
	// Which db to send the alert message
	DB int `toml:"db" overriede:"db"`
}

func NewConfig() Config {
	return Config{
		DB:    0,
		Queue: "_default_alert_queue_",
	}
}

func (c Config) Validate() error {
	if c.Enabled && c.Addr == "" {
		return errors.New("must specify the redis address")
	}
	return nil
}
