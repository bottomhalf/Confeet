package model

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User represents a user document in MongoDB
type User struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	UserID    string             `json:"userId" bson:"user_id"`
	Username  string             `json:"username" bson:"username"`
	Email     string             `json:"email" bson:"email"`
	FirstName string             `json:"firstName" bson:"first_name"`
	LastName  string             `json:"lastName" bson:"last_name"`
	Avatar    string             `json:"avatar" bson:"avatar"`
	Status    string             `json:"status" bson:"status"`
	IsActive  bool               `json:"isActive" bson:"is_active"`
	CreatedAt time.Time          `json:"createdAt" bson:"created_at"`
	UpdatedAt *time.Time         `json:"updatedAt" bson:"updated_at"`
	SyncedAt  *time.Time         `json:"syncedAt" bson:"synced_at"`
}
