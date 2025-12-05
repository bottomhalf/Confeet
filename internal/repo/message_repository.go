package repo

import (
	"Confeet/internal/db"
	"Confeet/internal/event"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

var (
	ErrMaxRetriesExceeded = errors.New("maximum retry attempts exceeded")
	ErrInvalidMessage     = errors.New("invalid message: message cannot be nil")
	ErrInvalidChannelID   = errors.New("invalid channel ID: cannot be empty")
	ErrOperationTimeout   = errors.New("operation timeout exceeded")
)

const (
	// Timeouts
	defaultWriteTimeout = 5 * time.Second
	defaultReadTimeout  = 10 * time.Second

	// Retry configuration
	maxRetries     = 3
	baseRetryDelay = 100 * time.Millisecond
	maxRetryDelay  = 2 * time.Second

	// Status constants
	StatusActive   = 1
	StatusInactive = 0
	StatusDeleted  = -1
)

type messageRepository struct {
	con       *mongo.Database
	mongoRepo *db.Repository[event.WsEvent]
	logger    *zap.Logger

	// for idempotency - track in-flight operations
	inFlightOps     map[string]struct{}
	inFlightOpsLock sync.RWMutex
}

type MessageRepository interface {
	InsertMessage(ctx context.Context, msg *event.WsEvent) (string, error)
}

func NewMessageRepository(mongo *mongo.Database, repo *db.Repository[event.WsEvent], logger *zap.Logger) MessageRepository {
	return &messageRepository{
		con:         mongo,
		mongoRepo:   repo,
		logger:      logger,
		inFlightOps: make(map[string]struct{}),
	}
}

// -----------------------------------------------------------------------------
// InsertMessage
// -----------------------------------------------------------------------------

func (m *messageRepository) InsertMessage(ctx context.Context, msg *event.WsEvent) (string, error) {
	err := m.validateMessage(msg)
	if err != nil {
		return "", err
	}

	ctx, cancel := m.ensureTimeout(ctx, defaultWriteTimeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			if err := m.waitForRetry(ctx, attempt); err != nil {
				return "", err
			}
		}

		result, err := m.mongoRepo.Create(ctx, *msg)
		if err != nil {
			m.logger.Info("message inserted successfully",
				zap.String("inserted_id", result.InsertedID.(string)),
				zap.String("channel_id", msg.ChannelId),
				zap.Int("attempt", attempt+1),
			)
			return result.InsertedID.(string), nil
		}

		lastErr = err

		// Don't retry on context cancellation or non-retryable errors
		if !m.isRetryableError(err) {
			break
		}

		m.logger.Warn("insert attempt failed, retrying",
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.Int("max_attempts", maxRetries),
		)
	}

	m.logger.Error("failed to insert message after all retries",
		zap.Error(lastErr),
		zap.String("channel_id", msg.ChannelId),
	)

	return "", fmt.Errorf("insert message failed: %w", lastErr)
}

// -----------------------------------------------------------------------------
// InsertMessageIdempotent - Prevents duplicate inserts
// -----------------------------------------------------------------------------

func (m *messageRepository) InsertMessageIdempotent(ctx context.Context, msg *event.WsEvent) (string, error) {
	if err := m.validateMessage(msg); err != nil {
		return "", err
	}

	key := m.generateIdempotencyKey(msg)

	// Check in-flight operations (race condition prevention)
	if !m.tryAcquireInFlight(key) {
		return "", fmt.Errorf("duplicate operation in progress: %s", key)
	}
	defer m.releaseInFlight(key)

	ctx, cancel := m.ensureTimeout(ctx, defaultWriteTimeout)
	defer cancel()

	// Check if already exists in DB
	filter := db.NewFilter().Eq("channelId", msg.ChannelId).Eq("messageId", msg.MessageId).Build()

	exists, err := m.mongoRepo.Exists(ctx, filter)
	if err != nil {
		return "", fmt.Errorf("existence check failed: %w", err)
	}

	if exists {
		existing, err := m.mongoRepo.FindOne(ctx, filter)
		if err != nil {
			return "", err
		}
		m.logger.Debug("message already exists", zap.String("id", existing.MessageId))
		return existing.ChannelId, nil
	}

	return m.InsertMessage(ctx, msg)
}

// -----------------------------------------------------------------------------
// FilterMessage
// -----------------------------------------------------------------------------
func (m *messageRepository) FilterMessage(ctx context.Context, channelId string, page int64) (*db.PaginatedResult[event.WsEvent], error) {
	if err := m.validateChannelId(channelId); err != nil {
		return nil, err
	}

	// Ensure timeout
	ctx, cancel := m.ensureTimeout(ctx, defaultReadTimeout)
	defer cancel()

	filter := db.NewFilter().Eq("channelId", channelId).Eq("status", 1).Build()
	result, err := m.mongoRepo.FindWithPagination(ctx, filter, db.PaginationParams{
		Page: page,
	})

	if err != nil {
		return nil, m.handleReadError(err, channelId)
	}

	m.logger.Debug("messages filtered successfully",
		zap.String("channel_id", channelId),
		zap.Int("count", len(result.Data)),
		zap.Int64("total", result.Total),
		zap.Int64("page", result.Page),
		zap.Int64("total_pages", result.TotalPages),
	)

	return result, nil
}

// -----------------------------------------------------------------------------
// Private Helper Methods
// -----------------------------------------------------------------------------

func (m *messageRepository) tryAcquireInFlight(key string) bool {
	m.inFlightOpsLock.Lock()
	defer m.inFlightOpsLock.Unlock()

	if _, exists := m.inFlightOps[key]; exists {
		return false
	}
	m.inFlightOps[key] = struct{}{}
	return true
}

func (m *messageRepository) releaseInFlight(key string) {
	m.inFlightOpsLock.Lock()
	defer m.inFlightOpsLock.Unlock()
	delete(m.inFlightOps, key)
}

func (m *messageRepository) generateIdempotencyKey(msg *event.WsEvent) string {
	if msg.MessageId != "" {
		return fmt.Sprintf("%s:%s", msg.ChannelId, msg.MessageId)
	}
	return fmt.Sprintf("%s:%d", msg.ChannelId, msg.MessageId)
}

func (m *messageRepository) validateMessage(msg *event.WsEvent) error {
	if msg == nil {
		return ErrInvalidMessage
	}
	if msg.ChannelId == "" {
		return ErrInvalidChannelID
	}
	return nil
}

func (m *messageRepository) ensureTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, hadDeadline := ctx.Deadline(); hadDeadline {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, timeout)
}

func (m *messageRepository) waitForRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(1<<uint(attempt)) * baseRetryDelay
	if delay > maxRetryDelay {
		delay = maxRetryDelay
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("retry wait cancelled: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func (m *messageRepository) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Context errors are not retryable
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}

	// Check for MongoDB transient errors
	if mongo.IsTimeout(err) || mongo.IsNetworkError(err) {
		return true
	}

	// Add more retryable error checks as needed
	return false
}

func (m *messageRepository) validateChannelId(channelId string) error {
	if channelId == "" {
		return ErrInvalidChannelID
	}
	return nil
}

func (m *messageRepository) handleReadError(err error, channelID string) error {
	if errors.Is(err, context.DeadlineExceeded) {
		m.logger.Error("read timeout", zap.String("channel_id", channelID))
		return ErrOperationTimeout
	}

	if errors.Is(err, context.Canceled) {
		m.logger.Debug("read cancelled", zap.String("channel_id", channelID))
		return err
	}

	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil // Not an error, just empty result
	}

	m.logger.Error("read failed", zap.Error(err), zap.String("channel_id", channelID))
	return fmt.Errorf("filter messages failed: %w", err)
}
