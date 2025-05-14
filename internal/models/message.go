package models

import (
	"time"
)

// MessageStatus represents the status of a message
type MessageStatus string

const (
	MessageStatusSent      MessageStatus = "sent"
	MessageStatusDelivered MessageStatus = "delivered"
	MessageStatusRead      MessageStatus = "read"
)

// Message represents a message between users
type Message struct {
	BaseModel
	SenderID   string        `gorm:"size:36;index" json:"senderId"`
	ReceiverID string        `gorm:"size:36;index" json:"receiverId"`
	ParentID   string        `gorm:"size:36;index" json:"parentId,omitempty"`
	Content    string        `gorm:"type:text" json:"content"`
	Subject    string        `gorm:"type:text" json:"subject"`
	Status     MessageStatus `gorm:"size:20;default:'sent'" json:"status"`
	ReadAt     *time.Time    `json:"readAt,omitempty"`

	// Relations
	Sender   User `gorm:"foreignKey:SenderID" json:"sender"`
	Receiver User `gorm:"foreignKey:ReceiverID" json:"receiver"`
}
