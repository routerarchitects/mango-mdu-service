package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/routerarchitects/mango-mdu-service/internal/services"
	"github.com/routerarchitects/ra-common-mods/apperror"
)

type SessionHandler struct {
	service     *services.SessionService
	AuthEnabled bool
}

func NewSessionHandler(service *services.SessionService, authEnabled bool) *SessionHandler {
	return &SessionHandler{
		service:     service,
		AuthEnabled: authEnabled,
	}
}

func (h *SessionHandler) Register(router fiber.Router) {
	router.Get("/session", h.GetSessionContext)
}

func (h *SessionHandler) GetSessionContext(c fiber.Ctx) error {
	token := c.Get("Authorization")
	if token == "" && h.AuthEnabled {
		return apperror.New(apperror.CodeUnauthorized, "unauthorized")
	}

	reqCtx := buildRequestContext(c)
	sess, err := h.service.GetSessionContext(reqCtx)
	if err != nil {
		return err
	}

	return c.JSON(sess)
}
