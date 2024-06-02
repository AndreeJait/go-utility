package redisw

import (
	"context"
	"fmt"
	"github.com/AndreeJait/go-utility/loggerw"
	"github.com/go-redis/redis/v8"
)

func ConnectToRedis(log loggerw.Logger, redisConfig RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", redisConfig.Host, redisConfig.Port),
		DB:       redisConfig.DB,
		Password: redisConfig.Password,
	})
	err := client.Ping(context.Background()).Err()
	if err != nil {
		return client, err
	}
	log.Infof("successfully connect to redis")
	return client, nil
}
