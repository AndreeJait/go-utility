package mongodb

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type nosql struct {
	client   *mongo.Client
	database *mongo.Database
}

func (n nosql) Aggregate(ctx context.Context, collectionName string, data []interface{}, pipeline []bson.M, opts ...*options.AggregateOptions) error {
	coll := n.database.Collection(collectionName)

	csr, err := coll.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return err
	}

	err = csr.All(ctx, data)
	return err
}

func (n nosql) Ping() {
	_ = n.client.Ping(context.Background(), nil)
}

func (n nosql) GetConnection() *mongo.Client {
	return n.client
}

func (n nosql) Insert(ctx context.Context, collectionName string, data interface{}, opts ...*options.InsertOneOptions) error {
	coll := n.database.Collection(collectionName)

	_, err := coll.InsertOne(ctx, data, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (n nosql) BulkInsert(ctx context.Context, collectionName string, data []interface{}, opts ...*options.InsertManyOptions) error {
	coll := n.database.Collection(collectionName)

	_, err := coll.InsertMany(ctx, data, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (n nosql) Update(ctx context.Context, collectionName string, selector bson.M, data bson.M, opts ...*options.UpdateOptions) error {
	coll := n.database.Collection(collectionName)

	_, err := coll.UpdateOne(ctx, selector, data, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (n nosql) BulkUpdate(ctx context.Context, collectionName string, selector bson.M, data bson.M, opts ...*options.UpdateOptions) error {
	coll := n.database.Collection(collectionName)

	_, err := coll.UpdateMany(ctx, selector, data, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (n nosql) Find(ctx context.Context, data interface{}, collectionName string, selector bson.M, opts ...*options.FindOneOptions) error {
	coll := n.database.Collection(collectionName)

	err := coll.FindOne(ctx, selector, opts...).Decode(data)
	if err != nil {
		return err
	}
	return nil
}

func (n nosql) FindAll(ctx context.Context, data interface{}, collectionName string, selector bson.M, opts ...*options.FindOptions) error {
	coll := n.database.Collection(collectionName)

	csr, err := coll.Find(ctx, selector, opts...)
	if err != nil {
		return err
	}
	err = csr.All(ctx, data)
	if err != nil {
		return err
	}
	return nil
}

func (n nosql) Delete(ctx context.Context, collectionName string, selector bson.M, opts ...*options.DeleteOptions) error {
	coll := n.database.Collection(collectionName)
	_, err := coll.DeleteOne(ctx, selector, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (n nosql) DeleteAll(ctx context.Context, collectionName string, selector bson.M, opts ...*options.DeleteOptions) error {
	coll := n.database.Collection(collectionName)
	_, err := coll.DeleteMany(ctx, selector, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (n nosql) Disconnect() error {
	return n.client.Disconnect(context.Background())
}

func New(uri string, dbName string) (NoSql, error) {
	serverApi := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().
		ApplyURI(uri).
		SetServerAPIOptions(serverApi)
	client, err := mongo.Connect(context.Background(), opts)
	if err != nil {
		return nil, err
	}
	db := client.Database(dbName)
	return &nosql{client: client, database: db}, nil
}
