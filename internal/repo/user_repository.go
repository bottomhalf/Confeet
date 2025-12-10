package repo

import (
	"Confeet/internal/db"
	"Confeet/internal/model"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type UserRepository interface {
	// Define methods for user repository here
	insertMessage(msg model.Message) error
}

type userRepository struct {
	// Add fields for dependencies here
	con       *mongo.Database
	mongoRepo *db.Repository[model.Message]
}

func NewUserRepository(con *mongo.Database, repo *db.Repository[model.Message]) UserRepository {
	return &userRepository{
		con:       con,
		mongoRepo: repo,
	}
}

func (r *userRepository) insertMessage(msg model.Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	result, err := r.mongoRepo.Create(ctx, msg)
	if err != nil {
		return err
	}

	fmt.Println("Inserted ID:", result.InsertedID)

	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("insert message timeout: %w", ctx.Err())
		}
	}

	return nil
}
