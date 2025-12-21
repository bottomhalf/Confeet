package repo

import (
	"Confeet/internal/model"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type conversationRepository struct {
	con    *mongo.Database
	logger *zap.Logger
}

type ConversationRepository interface {
	GetRoomDetail(ctx context.Context, conversationID string) (*model.Conversation, error)
}

func NewConversationRepository(mongo *mongo.Database, logger *zap.Logger) ConversationRepository {
	return &conversationRepository{
		con:    mongo,
		logger: logger,
	}
}

// GetRoomDetail fetches a conversation document by ID and returns it as a Conversation object
func (r *conversationRepository) GetRoomDetail(ctx context.Context, conversationID string) (*model.Conversation, error) {
	if conversationID == "" {
		return nil, ErrInvalidChannelID
	}

	// Ensure timeout
	ctx, cancel := r.ensureTimeout(ctx, defaultReadTimeout)
	defer cancel()

	// Convert string ID to ObjectID
	objectID, err := primitive.ObjectIDFromHex(conversationID)
	if err != nil {
		r.logger.Error("invalid conversation ID format",
			zap.String("conversation_id", conversationID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("invalid conversation ID format: %w", err)
	}

	collection := r.con.Collection("conversations")

	var conversation model.Conversation
	err = collection.FindOne(ctx, bson.M{"_id": objectID}).Decode(&conversation)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			r.logger.Debug("conversation not found",
				zap.String("conversation_id", conversationID),
			)
			return nil, nil
		}
		r.logger.Error("failed to fetch conversation",
			zap.String("conversation_id", conversationID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to fetch conversation: %w", err)
	}

	r.logger.Debug("conversation retrieved successfully",
		zap.String("conversation_id", conversationID),
		zap.Int("participants_count", len(conversation.Participants)),
	)

	return &conversation, nil
}

func (r *conversationRepository) ensureTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, hadDeadline := ctx.Deadline(); hadDeadline {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, timeout)
}
