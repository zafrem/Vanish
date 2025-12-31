package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/milkiss/vanish/backend/internal/models"
)

// MetadataRepository handles message metadata operations
type MetadataRepository struct {
	db *sql.DB
}

// NewMetadataRepository creates a new metadata repository
func NewMetadataRepository(db *sql.DB) *MetadataRepository {
	return &MetadataRepository{db: db}
}

// Create creates a new message metadata record
func (r *MetadataRepository) Create(ctx context.Context, metadata *models.MessageMetadata) error {
	query := `
		INSERT INTO message_metadata (message_id, sender_id, recipient_id, encryption_key, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	err := r.db.QueryRowContext(ctx, query,
		metadata.MessageID,
		metadata.SenderID,
		metadata.RecipientID,
		metadata.EncryptionKey,
		metadata.Status,
		metadata.CreatedAt,
		metadata.ExpiresAt,
	).Scan(&metadata.ID)

	if err != nil {
		return fmt.Errorf("failed to create metadata: %w", err)
	}

	return nil
}

// FindByMessageID finds metadata by message ID
func (r *MetadataRepository) FindByMessageID(ctx context.Context, messageID string) (*models.MessageMetadata, error) {
	query := `
		SELECT id, message_id, sender_id, recipient_id, status, created_at, read_at, expires_at
		FROM message_metadata
		WHERE message_id = $1
	`

	metadata := &models.MessageMetadata{}
	err := r.db.QueryRowContext(ctx, query, messageID).Scan(
		&metadata.ID,
		&metadata.MessageID,
		&metadata.SenderID,
		&metadata.RecipientID,
		&metadata.Status,
		&metadata.CreatedAt,
		&metadata.ReadAt,
		&metadata.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, models.ErrMessageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find metadata: %w", err)
	}

	return metadata, nil
}

// MarkAsRead marks a message as read
func (r *MetadataRepository) MarkAsRead(ctx context.Context, messageID string) error {
	query := `
		UPDATE message_metadata
		SET status = $1, read_at = $2
		WHERE message_id = $3
	`

	result, err := r.db.ExecContext(ctx, query, models.StatusRead, time.Now(), messageID)
	if err != nil {
		return fmt.Errorf("failed to mark as read: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return models.ErrMessageNotFound
	}

	return nil
}

// GetUserHistory returns message history for a user (sent or received)
func (r *MetadataRepository) GetUserHistory(ctx context.Context, userID int64, limit int) ([]*models.MessageHistoryResponse, error) {
	query := `
		SELECT
			m.message_id,
			sender.name as sender_name,
			recipient.name as recipient_name,
			m.status,
			m.created_at,
			m.read_at,
			m.expires_at,
			m.sender_id,
			m.recipient_id,
			m.encryption_key
		FROM message_metadata m
		JOIN users sender ON m.sender_id = sender.id
		JOIN users recipient ON m.recipient_id = recipient.id
		WHERE m.sender_id = $1 OR m.recipient_id = $1
		ORDER BY m.created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	defer rows.Close()

	var history []*models.MessageHistoryResponse
	for rows.Next() {
		h := &models.MessageHistoryResponse{}
		var senderID, recipientID int64
		var encryptionKey sql.NullString

		err := rows.Scan(
			&h.MessageID,
			&h.SenderName,
			&h.RecipientName,
			&h.Status,
			&h.CreatedAt,
			&h.ReadAt,
			&h.ExpiresAt,
			&senderID,
			&recipientID,
			&encryptionKey,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan history: %w", err)
		}

		h.IsSender = senderID == userID
		h.IsRecipient = recipientID == userID

		// Only include encryption key for recipients with pending messages
		if h.IsRecipient && h.Status == models.StatusPending && encryptionKey.Valid {
			h.EncryptionKey = encryptionKey.String
		}

		history = append(history, h)
	}

	return history, nil
}

// CleanupExpired marks expired messages as expired (called by cron job)
func (r *MetadataRepository) CleanupExpired(ctx context.Context) (int64, error) {
	query := `
		UPDATE message_metadata
		SET status = $1
		WHERE status = $2 AND expires_at < NOW()
	`

	result, err := r.db.ExecContext(ctx, query, models.StatusExpired, models.StatusPending)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup expired: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	return rows, nil
}
