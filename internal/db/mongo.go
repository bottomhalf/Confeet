package db

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PaginationParams holds pagination configuration
type PaginationParams struct {
	Page     int64  `json:"page"`     // Current page (1-based)
	PageSize int64  `json:"pageSize"` // Items per page
	SortBy   string `json:"sortBy"`   // Field to sort by
	SortDesc bool   `json:"sortDesc"` // Sort descending if true
}

// PaginatedResult holds paginated query results
type PaginatedResult[T any] struct {
	Data       []T   `json:"data"`
	Total      int64 `json:"total"`
	Page       int64 `json:"page"`
	PageSize   int64 `json:"pageSize"`
	TotalPages int64 `json:"totalPages"`
}

// Repository provides generic CRUD operations for MongoDB
type Repository[T any] struct {
	collection *mongo.Collection
}

// NewRepository creates a new generic repository
func NewRepository[T any](db *mongo.Database, collectionName string) *Repository[T] {
	return &Repository[T]{
		collection: db.Collection(collectionName),
	}
}

func OpenConnection(uri string, database string) (*mongo.Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(uri)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	return client.Database(database), nil
}

// Create inserts a new document
func (r *Repository[T]) Create(ctx context.Context, document T) (*mongo.InsertOneResult, error) {
	return r.collection.InsertOne(ctx, document)
}

// CreateMany inserts multiple documents
func (r *Repository[T]) CreateMany(ctx context.Context, documents []T) (*mongo.InsertManyResult, error) {
	docs := make([]interface{}, len(documents))
	for i, d := range documents {
		docs[i] = d
	}
	return r.collection.InsertMany(ctx, docs)
}

// FindByID finds a document by its ObjectID
func (r *Repository[T]) FindByID(ctx context.Context, id string) (*T, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var result T
	err = r.collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FindOne finds a single document matching the filter
func (r *Repository[T]) FindOne(ctx context.Context, filter bson.M) (*T, error) {
	var result T
	err := r.collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// FindAll finds all documents matching the filter
func (r *Repository[T]) FindAll(ctx context.Context, filter bson.M) ([]T, error) {
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []T
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// FindWithPagination finds documents with pagination support
func (r *Repository[T]) FindWithPagination(ctx context.Context, filter bson.M, params PaginationParams) (*PaginatedResult[T], error) {
	// Set defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 10
	}
	if params.PageSize > 100 {
		params.PageSize = 100 // Max limit
	}

	// Count total documents
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Calculate skip
	skip := (params.Page - 1) * params.PageSize

	// Build find options
	findOptions := options.Find()
	findOptions.SetSkip(skip)
	findOptions.SetLimit(params.PageSize)

	// Set sort order
	if params.SortBy != "" {
		sortOrder := 1
		if params.SortDesc {
			sortOrder = -1
		}
		findOptions.SetSort(bson.D{{Key: params.SortBy, Value: sortOrder}})
	}

	// Execute query
	cursor, err := r.collection.Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []T
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// Calculate total pages
	totalPages := total / params.PageSize
	if total%params.PageSize > 0 {
		totalPages++
	}

	return &PaginatedResult[T]{
		Data:       results,
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}, nil
}

// Update updates a single document matching the filter
func (r *Repository[T]) Update(ctx context.Context, filter bson.M, update bson.M) (*mongo.UpdateResult, error) {
	return r.collection.UpdateOne(ctx, filter, bson.M{"$set": update})
}

// UpdateByID updates a document by its ObjectID
func (r *Repository[T]) UpdateByID(ctx context.Context, id string, update bson.M) (*mongo.UpdateResult, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	return r.collection.UpdateOne(ctx, bson.M{"_id": objectID}, bson.M{"$set": update})
}

// UpdateMany updates multiple documents matching the filter
func (r *Repository[T]) UpdateMany(ctx context.Context, filter bson.M, update bson.M) (*mongo.UpdateResult, error) {
	return r.collection.UpdateMany(ctx, filter, bson.M{"$set": update})
}

// Delete deletes a single document matching the filter
func (r *Repository[T]) Delete(ctx context.Context, filter bson.M) (*mongo.DeleteResult, error) {
	return r.collection.DeleteOne(ctx, filter)
}

// DeleteByID deletes a document by its ObjectID
func (r *Repository[T]) DeleteByID(ctx context.Context, id string) (*mongo.DeleteResult, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	return r.collection.DeleteOne(ctx, bson.M{"_id": objectID})
}

// DeleteMany deletes multiple documents matching the filter
func (r *Repository[T]) DeleteMany(ctx context.Context, filter bson.M) (*mongo.DeleteResult, error) {
	return r.collection.DeleteMany(ctx, filter)
}

// Count counts documents matching the filter
func (r *Repository[T]) Count(ctx context.Context, filter bson.M) (int64, error) {
	return r.collection.CountDocuments(ctx, filter)
}

// Exists checks if a document matching the filter exists
func (r *Repository[T]) Exists(ctx context.Context, filter bson.M) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, filter, options.Count().SetLimit(1))
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
