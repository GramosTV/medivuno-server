package handlers

import (
	"fmt"
	"healthcare-app-server/internal/middleware"
	"healthcare-app-server/internal/models"
	"healthcare-app-server/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageHandler handles messaging related requests.
type MessageHandler struct {
	DB *gorm.DB
	// Potentially add a WebSocket upgrader here if using WebSockets for real-time
}

// NewMessageHandler creates a new MessageHandler.
func NewMessageHandler(db *gorm.DB) *MessageHandler {
	return &MessageHandler{DB: db}
}

// SendMessageRequest represents the request body for sending a message.
type SendMessageRequest struct {
	RecipientID     string `json:"recipientId" binding:"required,uuid"`
	Content         string `json:"content" binding:"required"`
	Subject         string `json:"subject"`
	ParentMessageID string `json:"parentMessageId"`
}

// SendMessage handles sending a new message.
func (h *MessageHandler) SendMessage(c *gin.Context) {
	var req SendMessageRequest
	if !utils.BindAndValidate(c, &req) {
		return
	}

	senderIDStr, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		utils.Unauthorized(c, "Sender ID not found in token")
		return
	}
	senderID, err := uuid.Parse(senderIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Sender ID format in token")
		return
	}

	recipientID, err := uuid.Parse(req.RecipientID)
	if err != nil {
		utils.BadRequest(c, "Invalid Recipient ID format")
		return
	}

	if senderID == recipientID {
		utils.BadRequest(c, "Cannot send a message to yourself.")
		return
	}

	// Verify recipient exists
	var recipient models.User
	if err := h.DB.First(&recipient, "id = ?", recipientID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Recipient user not found")
		} else {
			utils.InternalServerError(c, "Database error verifying recipient: "+err.Error())
		}
		return
	}
	// Verify sender exists (though middleware should ensure this)
	var sender models.User
	if err := h.DB.First(&sender, "id = ?", senderID).Error; err != nil {
		utils.NotFound(c, "Sender user not found") // Should not happen if auth middleware is correct
		return
	}

	// Authorization: Who can message whom?
	// For now, let's assume:
	// - Patients can message Doctors.
	// - Doctors can message Patients.
	// - Admins can message anyone (or this is handled differently).
	// - Patients cannot message other Patients directly (unless specified).
	// - Doctors cannot message other Doctors directly (unless specified).
	senderRole, _ := middleware.GetUserRoleFromContext(c)
	recipientRole := recipient.Role

	// Convert roles to lowercase for case-insensitive comparison
	senderRoleLower := strings.ToLower(string(senderRole))
	recipientRoleLower := strings.ToLower(string(recipientRole))

	// Log roles for debugging
	fmt.Printf("Sender Role: %s, Recipient Role: %s\n", senderRoleLower, recipientRoleLower)

	// Authorization logic for messaging
	allowedToMessage := false
	if (strings.Contains(senderRoleLower, "patient") && strings.Contains(recipientRoleLower, "doctor")) ||
		(strings.Contains(senderRoleLower, "doctor") && strings.Contains(recipientRoleLower, "patient")) {
		allowedToMessage = true
	}
	// Add more rules if Admins can message, or Doctor-to-Doctor, Patient-to-Patient allowed
	if strings.Contains(senderRoleLower, "admin") || strings.Contains(recipientRoleLower, "admin") {
		allowedToMessage = true
	}

	if !allowedToMessage {
		fmt.Printf("Message denied: Sender Role=%s, Recipient Role=%s\n", senderRole, recipientRole)
		utils.Forbidden(c, "You are not authorized to send a message to this user.")
		return
	}

	message := models.Message{
		SenderID:   senderID.String(),    // Convert UUID to string
		ReceiverID: recipientID.String(), // Convert UUID to string
		Content:    req.Content,
		Subject:    req.Subject,              // Save the message subject
		Status:     models.MessageStatusSent, // Default status
	}

	// If there's a parent message ID, try to set it
	if req.ParentMessageID != "" {
		if _, err := uuid.Parse(req.ParentMessageID); err == nil {
			message.ParentID = req.ParentMessageID
		}
	}

	if err := h.DB.Create(&message).Error; err != nil {
		utils.InternalServerError(c, "Failed to send message: "+err.Error())
		return
	}

	// Here you might trigger a real-time event (e.g., WebSocket push)

	utils.Created(c, "Message sent successfully", message)
}

// GetMessagesForUser handles fetching messages for the logged-in user (conversation list or specific conversation).
// This could be complex depending on how conversations are structured.
// A simple approach: get all messages where the user is sender or recipient.
func (h *MessageHandler) GetMessagesForUser(c *gin.Context) {
	userIDStr, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID, _ := uuid.Parse(userIDStr) // Assume valid UUID from token

	// Optional: Get messages with a specific other user (conversation view)
	otherUserIDStr := c.Query("withUser")
	var messages []models.Message

	query := h.DB.Preload("Sender").Preload("Receiver").Order("created_at asc")

	if otherUserIDStr != "" {
		otherUserID, err := uuid.Parse(otherUserIDStr)
		if err != nil {
			utils.BadRequest(c, "Invalid 'withUser' ID format")
			return
		}
		query = query.Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
			userID, otherUserID, otherUserID, userID)
	} else {
		// Get all messages involving the user (can be a lot, consider pagination)
		query = query.Where("sender_id = ? OR receiver_id = ?", userID, userID)
	}

	if err := query.Find(&messages).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch messages: "+err.Error())
		return
	} // Mark messages as "read" if the current user is the recipient
	// This is a simplified approach. A more robust system would track read status per user per message.
	for i, msg := range messages {
		if msg.ReceiverID == userID.String() && msg.Status == models.MessageStatusSent {
			messages[i].Status = models.MessageStatusRead
			h.DB.Model(&messages[i]).Update("status", models.MessageStatusRead) // Update in DB
		}
	}

	utils.Success(c, "Messages fetched successfully", messages)
}

// GetConversations handles fetching a list of conversations for the user.
// A conversation is typically defined by unique pairs of (user, other_user).
func (h *MessageHandler) GetConversations(c *gin.Context) {
	userIDStr, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID, _ := uuid.Parse(userIDStr)

	// This query is a bit more complex. It aims to get the latest message for each conversation partner.
	// One way to do this is to get all distinct users the current user has messaged or received messages from,
	// then fetch the latest message for each of those pairs.
	// This is a simplified version and might need optimization for performance on large datasets.

	var conversationPartners []struct {
		PartnerID uuid.UUID `gorm:"column:partner_id"`
	}

	// Find all distinct users the current user has interacted with
	// This subquery logic might need to be adjusted based on exact SQL dialect and GORM capabilities for complex distinct pairs
	err := h.DB.Raw(`
		SELECT DISTINCT partner_id FROM (
			SELECT receiver_id as partner_id FROM messages WHERE sender_id = ?
			UNION
			SELECT sender_id as partner_id FROM messages WHERE receiver_id = ?
		) AS partners
	`, userID, userID).Scan(&conversationPartners).Error

	if err != nil {
		utils.InternalServerError(c, "Failed to fetch conversation partners: "+err.Error())
		return
	}

	type ConversationPreview struct {
		Partner     models.UserSanitized `json:"partner"`
		LastMessage models.Message       `json:"lastMessage"`
		UnreadCount int64                `json:"unreadCount"`
	}
	var previews []ConversationPreview

	for _, cp := range conversationPartners {
		var partnerUser models.User
		if err := h.DB.First(&partnerUser, "id = ?", cp.PartnerID).Error; err != nil {
			continue // Skip if partner user not found
		}

		var lastMessage models.Message
		err := h.DB.Preload("Sender").Preload("Receiver").
			Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
				userID, cp.PartnerID, cp.PartnerID, userID).
			Order("created_at desc").First(&lastMessage).Error
		if err != nil {
			continue // Skip if no message found (should not happen if they are a partner)
		}

		var unreadCount int64
		h.DB.Model(&models.Message{}).
			Where("sender_id = ? AND receiver_id = ? AND status = ?", cp.PartnerID, userID, models.MessageStatusSent).
			Count(&unreadCount)

		previews = append(previews, ConversationPreview{
			Partner:     partnerUser.Sanitize(),
			LastMessage: lastMessage,
			UnreadCount: unreadCount,
		})
	}

	utils.Success(c, "Conversations fetched successfully", previews)
}

// MarkMessageAsRead handles marking a specific message as read.
// This is more granular than the automatic marking in GetMessagesForUser.
func (h *MessageHandler) MarkMessageAsRead(c *gin.Context) {
	messageIDStr := c.Param("messageId")
	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		utils.BadRequest(c, "Invalid Message ID format")
		return
	}

	userIDStr, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	userID, _ := uuid.Parse(userIDStr)

	var message models.Message
	if err := h.DB.First(&message, "id = ?", messageID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Message not found")
		} else {
			utils.InternalServerError(c, "Database error: "+err.Error())
		}
		return
	}
	// Only the recipient can mark a message as read
	if message.ReceiverID != userID.String() {
		utils.Forbidden(c, "You are not authorized to mark this message as read.")
		return
	}

	if message.Status == models.MessageStatusRead {
		utils.Success(c, "Message already marked as read", message)
		return
	}

	message.Status = models.MessageStatusRead
	if err := h.DB.Save(&message).Error; err != nil {
		utils.InternalServerError(c, "Failed to update message status: "+err.Error())
		return
	}

	utils.Success(c, "Message marked as read successfully", message)
}

// NewMessagesRequest represents the query params for getting new messages
type NewMessagesRequest struct {
	Since string `form:"since" binding:"required"`
}

// GetNewMessages handles fetching new messages since a given timestamp
func (h *MessageHandler) GetNewMessages(c *gin.Context) {
	var req NewMessagesRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	userID, exists := middleware.GetUserIDFromContext(c)
	if !exists {
		utils.Unauthorized(c, "User ID not found in context")
		return
	}

	// Parse the since timestamp
	sinceTime, err := time.Parse(time.RFC3339, req.Since)
	if err != nil {
		utils.BadRequest(c, "Invalid timestamp format. Use RFC3339 format (e.g., 2006-01-02T15:04:05Z07:00)")
		return
	}

	// Get messages received after the specified time
	var messages []models.Message
	if err := h.DB.Preload("Sender").Preload("Receiver").
		Where("(receiver_id = ? OR sender_id = ?) AND created_at > ?", userID, userID, sinceTime).
		Order("created_at DESC").
		Find(&messages).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch messages: "+err.Error())
		return
	}

	utils.Success(c, "New messages fetched successfully", messages)
}
