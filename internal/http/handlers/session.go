package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type SessionHandler struct {
	service *services.SessionService
}

func NewSessionHandler(service *services.SessionService) *SessionHandler {
	return &SessionHandler{
		service: service,
	}
}

func (h *SessionHandler) Register(router fiber.Router) {
	router.Get("/session", h.GetSessionContext)
}

func (h *SessionHandler) GetSessionContext(c fiber.Ctx) error {
	token := c.Get("Authorization")
	if token == "" {
		return apperror.New(apperror.CodeUnauthorized, "unauthorized")
	}

	ctx := c.Context()
	sess, err := h.service.GetSessionContext(ctx, token)
	if err != nil {
		return err
	}

	return c.JSON(sess)
}
