package mongow

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestDebugContextInjection verifies that the debug flag is correctly stored in the context.
func TestDebugContextInjection(t *testing.T) {
	ctx := context.Background()

	if val := ctx.Value(debugKey); val != nil {
		t.Errorf("Expected initial debug context to be nil, got %v", val)
	}

	debugCtx := DebugContext(ctx)
	if val, ok := debugCtx.Value(debugKey).(bool); !ok || !val {
		t.Errorf("Expected debug context to contain 'true', got %v", val)
	}
}

// TestMongoConnectionAndMonitor tests connectivity and the command logging hook.
func TestMongoConnectionAndMonitor(t *testing.T) {
	cfg := &Config{
		URI:       "mongodb://localhost:27017",
		DebugMode: false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := Connect(ctx, cfg)
	if err != nil {
		t.Skipf("Skipping integration test: MongoDB not reachable: %v", err)
		return
	}
	defer func() {
		_ = Disconnect(client)(context.Background())
	}()

	db := client.Database("test_db")
	coll := db.Collection("test_collection")

	// Use DebugContext to trigger logs for this specific call
	debugCtx := DebugContext(ctx)
	doc := bson.M{"test_key": "v2_driver", "at": time.Now()}

	_, err = coll.InsertOne(debugCtx, doc)
	if err != nil {
		t.Fatalf("InsertOne failed: %v", err)
	}

	// Clean up
	_, err = coll.DeleteMany(ctx, bson.M{"test_key": "v2_driver"})
	if err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}
}

// TestMongoTransaction tests the transaction wrapper.
// It handles failures gracefully if the local DB is not a Replica Set.
func TestMongoTransaction(t *testing.T) {
	cfg := &Config{URI: "mongodb://localhost:27017"}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := Connect(ctx, cfg)
	if err != nil {
		t.Skip("Skipping: MongoDB unreachable")
		return
	}
	defer func() { _ = Disconnect(client)(context.Background()) }()

	err = Transaction(ctx, client, func(sessCtx context.Context) error {
		coll := client.Database("test_db").Collection("tx_test")
		_, err := coll.InsertOne(sessCtx, bson.M{"active": true})
		return err
	})

	if err != nil {
		// Specific handling for non-replica set environments
		if isNoTransactionError(err) {
			t.Log("Skipping transaction test: Standalone MongoDB detected.")
			return
		}
		t.Fatalf("Transaction failed: %v", err)
	}

	// Cleanup
	_ = client.Database("test_db").Collection("tx_test").Drop(ctx)
}

// isNoTransactionError checks if the error is due to transactions not being supported.
func isNoTransactionError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "replica set") ||
		strings.Contains(msg, "Standalone") ||
		strings.Contains(msg, "featureSessionAndTransaction")
}
