package rest

import (
	"context"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/s3"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/utils"
	"github.com/gofiber/fiber/v2"
)

type S3Settings struct {
	// No service needed - direct config access
}

func InitRestS3Settings(app fiber.Router) S3Settings {
	rest := S3Settings{}
	app.Get("/settings/s3", rest.GetS3Config)
	app.Put("/settings/s3", rest.UpdateS3Config)
	app.Post("/settings/s3/test", rest.TestS3Connection)
	return rest
}

// GetS3Config returns the current S3 configuration
func (handler *S3Settings) GetS3Config(c *fiber.Ctx) error {
	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "S3 configuration retrieved",
		Results: config.S3,
	})
}

// UpdateS3Config updates S3 configuration and reinitializes the client
func (handler *S3Settings) UpdateS3Config(c *fiber.Ctx) error {
	var req config.S3Config
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Code:    "INVALID_REQUEST",
			Message: "Invalid request body: " + err.Error(),
		})
	}

	// Update global config
	config.S3 = req

	// Reinitialize S3 client with new config
	if err := s3.Reinitialize(req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.ResponseData{
			Code:    "S3_INIT_ERROR",
			Message: "Failed to initialize S3 client: " + err.Error(),
		})
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "S3 configuration updated successfully",
	})
}

// TestS3Connection tests S3 connection without saving the configuration
func (handler *S3Settings) TestS3Connection(c *fiber.Ctx) error {
	var req config.S3Config
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
			Code:    "INVALID_REQUEST",
			Message: "Invalid request body: " + err.Error(),
		})
	}

	// Create temporary client for testing
	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()

	// Test by trying to list bucket (head bucket)
	if req.Enabled {
		if err := s3.TestConnection(ctx, req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(utils.ResponseData{
				Code:    "S3_TEST_FAILED",
				Message: "S3 connection test failed: " + err.Error(),
			})
		}
	}

	return c.JSON(utils.ResponseData{
		Status:  200,
		Code:    "SUCCESS",
		Message: "S3 connection test passed",
	})
}
