package nsqw

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/AndreeJait/go-utility/loggerw"
	"github.com/nsqio/go-nsq"
	"testing"
)

func TestNsqW(t *testing.T) {
	t.Run("handle nsq running", func(t *testing.T) {
		log, _ := loggerw.DefaultLog()
		nsqW := New(Config{
			Hosts: []Host{
				{
					Host: "127.0.0.1",
					Port: "4161",
				},
			},
		}, log)

		err := nsqW.AddHandler(AddHandlerParam{
			Topic:                 "testing_andree_handler",
			Channel:               "andree_utility",
			MaxAttempts:           10,
			MaxInFlight:           5,
			MaxRequeueInDelay:     900,
			DefaultRequeueInDelay: 0,
			Handler: func(ctx context.Context, message *nsq.Message) error {
				var reqMap = map[string]interface{}{}
				if err := json.Unmarshal(message.Body, &reqMap); err != nil {
					return err
				}

				jsonStr, _ := json.Marshal(reqMap)
				log.Info(ctx, string(jsonStr))
				return nil
			},
		})

		err = nsqW.AddHandler(AddHandlerParam{
			Topic:                 "testing_andree_handler_3",
			Channel:               "andree_utility",
			MaxAttempts:           10,
			MaxInFlight:           5,
			MaxRequeueInDelay:     900,
			DefaultRequeueInDelay: 0,
			Handler: func(ctx context.Context, message *nsq.Message) error {
				var reqMap = map[string]interface{}{}
				if err := json.Unmarshal(message.Body, &reqMap); err != nil {
					return err
				}

				jsonStr, _ := json.Marshal(reqMap)
				log.Info(ctx, string(jsonStr))
				return nil
			},
		})

		err = nsqW.AddHandler(AddHandlerParam{
			Topic:                 "testing_andree_handler_2",
			Channel:               "andree_utility_2",
			MaxAttempts:           10,
			MaxInFlight:           5,
			MaxRequeueInDelay:     900,
			DefaultRequeueInDelay: 0,
			Handler: func(ctx context.Context, message *nsq.Message) error {
				var reqMap = map[string]interface{}{}
				if err := json.Unmarshal(message.Body, &reqMap); err != nil {
					return err
				}

				jsonStr, _ := json.Marshal(reqMap)
				log.Info(ctx, string(jsonStr))
				return nil
			},
		})

		if err != nil {
			fmt.Println(err)
			return
		}

		err = nsqW.Start()
		if err != nil {
			fmt.Println(err)
			return
		}
	})
}
