package routes

import (
	"healthcare-app-server/internal/config"
	"healthcare-app-server/internal/handlers"
	"healthcare-app-server/internal/middleware"
	"healthcare-app-server/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupRoutes configures the application routes.
func SetupRoutes(router *gin.Engine, db *gorm.DB, cfg *config.Config) {
	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db, cfg)
	userHandler := handlers.NewUserHandler(db)
	appointmentHandler := handlers.NewAppointmentHandler(db)
	medicalRecordHandler := handlers.NewMedicalRecordHandler(db)
	messageHandler := handlers.NewMessageHandler(db)

	// Public routes (no authentication required)
	public := router.Group("/api/v1")
	{
		authRoutes := public.Group("/auth")
		{
			authRoutes.POST("/register", authHandler.Register)
			authRoutes.POST("/login", authHandler.Login)
			authRoutes.POST("/refresh-token", authHandler.RefreshToken)
			// Logout can be here or in authenticated routes depending on if it needs to invalidate server-side session/token
		}
	}

	// Authenticated routes
	private := router.Group("/api/v1")
	private.Use(middleware.AuthMiddleware(cfg)) // Apply JWT authentication middleware
	{
		// Auth related (e.g., profile, logout if it needs auth)
		authRoutesPrivate := private.Group("/auth")
		{
			authRoutesPrivate.POST("/logout", authHandler.Logout) // Assuming logout might interact with user session
			authRoutesPrivate.GET("/profile", authHandler.GetProfile)
			authRoutesPrivate.PUT("/profile", authHandler.UpdateProfile)
		}
		// User management routes (typically admin-only)
		userRoutes := private.Group("/users")
		{
			// Special endpoint to get doctors - accessible by all authenticated users
			userRoutes.GET("/doctors", userHandler.GetDoctors)

			// Special endpoint to get patients for a doctor - accessible by doctors and admins
			userRoutes.GET("/doctor-patients", userHandler.GetDoctorPatients)

			// Admin-only routes
			adminRoutes := userRoutes.Group("")
			adminRoutes.Use(middleware.RoleAuthMiddleware(models.RoleAdmin)) // Only Admins
			{
				adminRoutes.POST("", userHandler.CreateUser)
				adminRoutes.GET("", userHandler.GetUsers)
				adminRoutes.GET("/:id", userHandler.GetUserByID)
				adminRoutes.PUT("/:id", userHandler.UpdateUser)
				adminRoutes.DELETE("/:id", userHandler.DeleteUser)
			}
		}

		// Appointment routes
		appointmentRoutes := private.Group("/appointments")
		{
			// Patients can create appointments for themselves
			// Doctors/Admins might also create appointments (adjust RoleAuthMiddleware if needed or handle in handler)
			appointmentRoutes.POST("", middleware.RoleAuthMiddleware(models.RolePatient, models.RoleDoctor, models.RoleAdmin), appointmentHandler.CreateAppointment)

			// All authenticated users can get their own appointments
			appointmentRoutes.GET("", appointmentHandler.GetAppointmentsForUser) // Logic inside handler differentiates by role

			// Specific appointment access (Patient involved, Doctor involved, or Admin)
			appointmentRoutes.GET("/:id", appointmentHandler.GetAppointmentByID) // Authorization inside handler

			// Status updates (Doctor, Admin, Patient for cancellation)
			appointmentRoutes.PATCH("/:id/status", appointmentHandler.UpdateAppointmentStatus) // Authorization inside handler

			// Reschedule (Doctor, Admin, Patient if allowed)
			appointmentRoutes.PATCH("/:id/reschedule", appointmentHandler.RescheduleAppointment) // Authorization inside handler
		}

		// Medical Record routes
		medicalRecordRoutes := private.Group("/medical-records")
		{
			// Doctors create medical records
			medicalRecordRoutes.POST("", middleware.RoleAuthMiddleware(models.RoleDoctor), medicalRecordHandler.CreateMedicalRecord)

			// Patient can get their own, Doctors can get for their patients (or any, depending on policy)
			medicalRecordRoutes.GET("/patient/:patientId", medicalRecordHandler.GetMedicalRecordsForPatient) // Auth in handler

			// Get specific record (Patient if theirs, Doctor if involved/theirs, Admin)
			medicalRecordRoutes.GET("/:id", medicalRecordHandler.GetMedicalRecordByID) // Auth in handler

			// Doctors update their records, Admins can update any
			medicalRecordRoutes.PUT("/:id", middleware.RoleAuthMiddleware(models.RoleDoctor, models.RoleAdmin), medicalRecordHandler.UpdateMedicalRecord) // Further auth in handler if needed (e.g. doctor owns record)

			// Doctors delete their records, Admins can delete any
			medicalRecordRoutes.DELETE("/:id", middleware.RoleAuthMiddleware(models.RoleDoctor, models.RoleAdmin), medicalRecordHandler.DeleteMedicalRecord) // Further auth in handler

			// Attachment routes for a specific medical record
			attachmentRoutes := medicalRecordRoutes.Group("/:id/attachments")
			attachmentRoutes.Use(middleware.RoleAuthMiddleware(models.RoleDoctor)) // Only Doctors can manage attachments
			{
				attachmentRoutes.POST("", medicalRecordHandler.UploadMedicalRecordAttachment)
				// Potentially add GET for listing attachments for a record, DELETE for an attachment, etc.
			}

			// New route to get a specific attachment by its own ID
			// This is outside the /:id/attachments group because attachment ID is globally unique
			// Accessible by users who have access to the parent medical record (handled in the handler)
			private.GET("/medical-records/attachments/:attachmentId", medicalRecordHandler.GetMedicalRecordAttachment)
		}
		// Messaging routes
		messageRoutes := private.Group("/messages")
		{
			// Authenticated users (Patient, Doctor) can send messages based on rules in handler
			messageRoutes.POST("/send", messageHandler.SendMessage)

			// Get messages for the current user (either all or with a specific user)
			messageRoutes.GET("", messageHandler.GetMessagesForUser) // Auth in handler

			// Get new messages since a specified timestamp
			messageRoutes.GET("/new", messageHandler.GetNewMessages) // Auth in handler

			// Get a list of conversations
			messageRoutes.GET("/conversations", messageHandler.GetConversations)      // Auth in handler			// Mark a specific message as read
			messageRoutes.PATCH("/:messageId/read", messageHandler.MarkMessageAsRead) // Auth in handler
		}

	}

	// Simple health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP"})
	})
}
