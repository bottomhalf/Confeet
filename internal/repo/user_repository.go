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
	GetUser(id string) (*model.User, error)
}

type userRepository struct {
	// Add fields for dependencies here
	con       *mongo.Database
	mongoRepo *db.Repository[model.User]
}

func NewUserRepository(con *mongo.Database, repo *db.Repository[model.User]) UserRepository {
	return &userRepository{
		con:       con,
		mongoRepo: repo,
	}
}

func (r *userRepository) GetUser(id string) (*model.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	result, err := r.mongoRepo.FindByUserID(ctx, id)
	if err != nil {
		return nil, err
	}

	fmt.Println("Client found with ID:", result.ID)

	return result, nil
}
