package redisw

type RedisConfig struct {
	Host     string `json:"host" yaml:"host"`
	Port     string `json:"port" yaml:"port"`
	Password string `json:"password" yaml:"password"`
	DB       int    `json:"db" yaml:"db"`
}
