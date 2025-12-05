package service

import "Confeet/internal/repo"

type UserService interface {
	// Define methods for user service here
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
