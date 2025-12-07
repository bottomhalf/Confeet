package service

import (
	"Confeet/internal/model"
	"Confeet/internal/repo"
	"context"
)

type UserService interface {
	// Define methods for user service here
	GetMeetingRooms(context.Context) []model.Conversation
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
