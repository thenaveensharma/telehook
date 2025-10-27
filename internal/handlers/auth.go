package handlers

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/thenaveensharma/telehook/internal/auth"
	"github.com/thenaveensharma/telehook/internal/database"
	"github.com/thenaveensharma/telehook/internal/models"
)

type AuthHandler struct {
	db *database.DB
}

func NewAuthHandler(db *database.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

func (h *AuthHandler) Signup(c *fiber.Ctx) error {
	var req models.SignupRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Validate required fields
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "username, email, and password are required",
		})
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		log.Printf("Error hashing password: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to process password",
		})
	}

	// Create user
	user, err := h.db.CreateUser(context.Background(), req.Username, req.Email, passwordHash)
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to create user, email or username may already exist",
		})
	}

	// Generate JWT
	token, err := auth.GenerateJWT(user.ID, user.Email, user.Username)
	if err != nil {
		log.Printf("Error generating JWT: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(models.LoginResponse{
		Token:        token,
		User:         *user,
		WebhookToken: user.WebhookToken,
	})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "email and password are required",
		})
	}

	// Get user by email
	user, err := h.db.GetUserByEmail(context.Background(), req.Email)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid email or password",
		})
	}

	// Verify password
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid email or password",
		})
	}

	// Generate JWT
	token, err := auth.GenerateJWT(user.ID, user.Email, user.Username)
	if err != nil {
		log.Printf("Error generating JWT: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to generate token",
		})
	}

	return c.JSON(models.LoginResponse{
		Token:        token,
		User:         *user,
		WebhookToken: user.WebhookToken,
	})
}
