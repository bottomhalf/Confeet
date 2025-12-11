package service

import (
	"Confeet/internal/db"
	"Confeet/internal/model"
	"Confeet/internal/repo"
	"context"
)

type UserService interface {
	// Define methods for user service here
	GetMeetingRooms(context.Context) []model.Conversation
	GetRoomMessagesService(ctx context.Context, conversationId string, page int64) (*db.PaginatedResult[model.Message], error)
}

type userService struct {
	// Add fields for dependencies here
	repo        repo.UserRepository
	messageRepo repo.MessageRepository
}

func NewUserService(repo repo.UserRepository, messageRepo repo.MessageRepository) UserService {
	return &userService{
		repo:        repo,
		messageRepo: messageRepo,
	}
}

func (s *userService) GetMeetingRooms(ctx context.Context) []model.Conversation {
	// Placeholder implementation
	msg, err := s.messageRepo.GetMeetingRooms(ctx)
	if err != nil {
		return []model.Conversation{}
	}

	return msg
}

func (s *userService) GetRoomMessagesService(ctx context.Context, conversationId string, page int64) (*db.PaginatedResult[model.Message], error) {
	// Placeholder implementation
	msg, err := s.messageRepo.FilterMessage(ctx, conversationId, page)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
