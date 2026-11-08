package handler

import (
	"crypto/rsa"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/model"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
	publicKey   *rsa.PublicKey
}

func NewAuthHandler(authService *service.AuthService, publicKey *rsa.PublicKey) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		publicKey:   publicKey,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, model.MsgInvalidBody)
		return
	}

	if req.Password == nil || *req.Password == "" || strings.TrimSpace(string(req.Email)) == "" {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, model.MsgEmailPasswordReq)
		return
	}

	ipAddress := c.ClientIP()
	var userAgent *string
	if ua := c.GetHeader("User-Agent"); ua != "" {
		userAgent = &ua
	}

	tokens, err := h.authService.Login(c.Request.Context(), string(req.Email), *req.Password, ipAddress, userAgent)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrInvalidCredentials):
			respondProblem(c, http.StatusUnauthorized, model.ErrCodeInvalidCredentials, model.MsgInvalidCredentials)
		case errors.Is(err, model.ErrAccountSuspended):
			respondProblem(c, http.StatusForbidden, model.ErrCodeAccountSuspended, model.MsgAccountSuspended)
		case errors.Is(err, model.ErrAccountPending):
			respondProblem(c, http.StatusForbidden, model.ErrCodeAccountPending, model.MsgAccountPending)
		default:
			respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		}
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  tokens.AccessToken,
		ExpiresIn:    tokens.ExpiresIn,
		RefreshToken: tokens.RefreshToken,
		TokenType:    Bearer,
	})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented yet"})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented yet"})
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented yet"})
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented yet"})
}
