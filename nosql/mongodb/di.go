package mongodb

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type NoSql interface {
	GetConnection() *mongo.Client

	Insert(ctx context.Context, collectionName string, data interface{}, opts ...*options.InsertOneOptions) error
	BulkInsert(ctx context.Context, collectionName string, data []interface{}, opts ...*options.InsertManyOptions) error

	Update(ctx context.Context, collectionName string, selector bson.M, data bson.M, opts ...*options.UpdateOptions) error
	BulkUpdate(ctx context.Context, collectionName string, selector bson.M, data bson.M, opts ...*options.UpdateOptions) error

	Find(ctx context.Context, data interface{}, collectionName string, selector bson.M, opts ...*options.FindOneOptions) error
	FindAll(ctx context.Context, data interface{}, collectionName string, selector bson.M, opts ...*options.FindOptions) error

	Delete(ctx context.Context, collectionName string, selector bson.M, opts ...*options.DeleteOptions) error
	DeleteAll(ctx context.Context, collectionName string, selector bson.M, opts ...*options.DeleteOptions) error

	Disconnect() error
	Ping()

	Aggregate(ctx context.Context, collectionName string, data []interface{}, pipeline []bson.M, opts ...*options.AggregateOptions) error
}
