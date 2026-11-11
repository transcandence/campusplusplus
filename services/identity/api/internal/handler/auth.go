package handler

import (
	"crypto/rsa"
	"errors"
	"log"
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
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil{
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Refresh token is required")
		return
	}
	ipAddress := c.ClientIP()
	var userAgent *string
	if ua := c.GetHeader("User-Agent"); ua != "" {
		userAgent = &ua
	}
	tokens, err := h.authService.Refresh(c.Request.Context(), req.RefreshToken, ipAddress, userAgent)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrInvalidRefresh):
			respondProblem(c, http.StatusUnauthorized, model.ErrCodeInvalidRefresh, "Invalid or expired refresh token")
		case errors.Is(err, model.ErrAccountSuspended):
			respondProblem(c, http.StatusForbidden, model.ErrCodeAccountSuspended, model.MsgAccountSuspended)
		default:
			log.Printf("ERROR refresh: %v", err)
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

func (h *AuthHandler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil{
		c.Status(http.StatusNoContent)
		return
	}
	_ = h.authService.Logout(c.Request.Context(), req.RefreshToken)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		respondProblem(c, http.StatusUnauthorized, model.ErrCodeUnauthorized, "Authentication required")
		return
	}
	userID, ok := userIDVal.(int64)
	if !ok {
		respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		return
	}
	if err := h.authService.LogoutAll(c.Request.Context(), userID); err != nil {
		respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		return 
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		respondProblem(c, http.StatusUnauthorized, model.ErrCodeUnauthorized, "Authentification required")
		return
	}
	userID, ok := userIDVal.(int64)
	if !ok {
		respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		return
	}
	userMe, err := h.authService.GetMe(c.Request.Context(), userID)
	if err != nil {
		switch{
		case errors.Is(err, model.ErrUserNotFound):
				respondProblem(c, http.StatusUnauthorized, model.ErrCodeUnauthorized, "User not found")
		default:
			respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		}
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, userMe)
}
