package redisw

import (
	"github.com/AndreeJait/go-utility/loggerw"
	"testing"
)

func TestConnectToRedis(t *testing.T) {
	type args struct {
		log         loggerw.Logger
		redisConfig RedisConfig
	}

	log, _ := loggerw.DefaultLog()

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success connect",
			args: args{
				log: log,
				redisConfig: RedisConfig{
					Host:     "localhost",
					Port:     "6379",
					Password: "",
					DB:       0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ConnectToRedis(tt.args.log, tt.args.redisConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectToRedis() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
