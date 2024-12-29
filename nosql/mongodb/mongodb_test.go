package mongodb

import (
	"context"
	"github.com/AndreeJait/go-utility/loggerw"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
)

func TestConnection(t *testing.T) {
	type Test struct {
		ID   *primitive.ObjectID `json:"id" bson:"_id"`
		Name string              `json:"name" bson:"name"`
	}
	t.Run("testing", func(t *testing.T) {
		log, _ := loggerw.DefaultLog()
		noSql, err := New("mongodb://root:andre110102@localhost:27017/", "test")
		if err != nil {
			log.Error(err)
			return
		}

		noSql.Ping()
		primitiveObjectID := primitive.NewObjectID()
		primitiveObjectID2 := primitive.NewObjectID()
		primitiveObjectID3 := primitive.NewObjectID()

		var example = Test{
			ID:   &primitiveObjectID,
			Name: "Andree 1",
		}
		var tempResultOne Test
		var tempResult []Test

		var exampleTests = []interface{}{
			Test{
				ID:   &primitiveObjectID2,
				Name: "Andree 2",
			},
			Test{
				ID:   &primitiveObjectID3,
				Name: "Andree 3",
			},
		}

		err = noSql.Insert(context.Background(), "names", example)
		if err != nil {
			log.Error(err)
			return
		}

		objectID := *example.ID
		err = noSql.Find(context.Background(), &tempResultOne, "names", bson.M{
			"_id": objectID,
		})
		if err != nil {
			log.Error(err)
			return
		}
		log.Infof("%+v", tempResultOne)

		err = noSql.BulkInsert(context.Background(), "names", exampleTests)
		if err != nil {
			log.Error(err)
			return
		}

		err = noSql.FindAll(context.Background(), &tempResult, "names", bson.M{})
		if err != nil {
			log.Error(err)
			return
		}
		objectID = *tempResult[0].ID
		log.Infof("%+v", tempResult)

		err = noSql.Update(context.Background(), "names", bson.M{"_id": objectID}, bson.M{
			"$set": map[string]interface{}{
				"name": "Andree 4",
			},
		})
		if err != nil {
			log.Error(err)
			return
		}

		err = noSql.BulkUpdate(context.Background(), "names", bson.M{}, bson.M{
			"$set": map[string]interface{}{
				"name": "Andree 5",
			},
		})
		if err != nil {
			log.Error(err)
			return
		}

		err = noSql.FindAll(context.Background(), &tempResult, "names", bson.M{})
		if err != nil {
			log.Error(err)
			return
		}
		log.Infof("%+v", tempResult)

		err = noSql.Delete(context.TODO(), "names", bson.M{"_id": objectID})
		if err != nil {
			log.Error(err)
			return
		}

		err = noSql.FindAll(context.Background(), &tempResult, "names", bson.M{})
		if err != nil {
			log.Error(err)
			return
		}
		log.Infof("%+v", tempResult)

		err = noSql.DeleteAll(context.TODO(), "names", bson.M{})
		if err != nil {
			log.Error(err)
			return
		}

		err = noSql.FindAll(context.Background(), &tempResult, "names", bson.M{})
		if err != nil {
			log.Error(err)
			return
		}
		log.Infof("%+v", tempResult)

		err = noSql.Disconnect()
		if err != nil {
			log.Error(err)
			return
		}
	})
}
