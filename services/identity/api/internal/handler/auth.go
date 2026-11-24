package handler

import (
	"crypto/rsa"
	"errors"
	"net/http"
	"strings"
	"log"

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
			respondProblem(c, http.StatusUnauthorized, model.ErrCodeInvalidRefresh, model.MsgInvalidRefresh)
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

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil{
				respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Invalid request body")
				return 
	}
	hasEmail := req.Email != nil && strings.TrimSpace(string(*req.Email)) != ""
	hasPhone := req.Phone != nil && strings.TrimSpace(*req.Phone) != ""

	if !hasEmail && !hasPhone {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "At least one of 'email' or 'phone' must be provided")
		return
	}
	if req.Password == nil || *req.Password == "" {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Password is required")
		return
	}
	var emailStr *string
	if req.Email != nil{
		e:= string(*req.Email)
		emailStr = &e
	}
	var genderStr *string
	if req.Gender != nil {
		g := string(*req.Gender)
		genderStr = &g
	}
	if len(*req.Password) < 8 || len(*req.Password) > 72 {
    	respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Password must be between 8 and 72 characters")
    	return
	}
	// Campus code is passed via subtypeAttributes.campus_code or defaults to "rabat".
	campusCodeStr := "rabat"
	if req.SubtypeAttributes != nil {
		if v, ok := (*req.SubtypeAttributes)["campus_code"]; ok {
			if s, ok := v.(string); ok && s != "" {
				campusCodeStr = s
			}
		}
	}
	campusID, err := h.authService.ResolveCampusID(c.Request.Context(), campusCodeStr)
	if err != nil {
	    respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Invalid campus code")
	    return
	}
	input := model.RegisterInput{
		FullName: 	req.FullName,
		Email: 		emailStr,
		Phone: 		req.Phone,
		Password: 	req.Password,
		UserType: 	string(req.UserType),
		Gender: 	genderStr,
		CampusID: 	campusID,
		// Affiliation: model.Affiliation, 
	}
	result, err := h.authService.Register(c.Request.Context(), input)
	if err != nil {
    	log.Printf("REGISTER ERROR: %+v", err)
		switch {
		case errors.Is(err, model.ErrUserAlreadyExists):
			respondProblem(c, http.StatusConflict, model.ErrCodeUserAlreadyExists, "An account with this email or phone already exists")
		case errors.Is(err, model.ErrInvalidRole):
			respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeInavlidRole, model.MsgInvalidRole)
		default:
			respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		}
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusCreated, result)
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
			respondProblem(c, http.StatusUnauthorized, model.ErrCodeInvalidRefresh, "Invalid or expired refresh token")
		default:
			respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		}
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, TokenResponse{
		AccessToken: tokens.AccessToken,
		ExpiresIn: tokens.ExpiresIn,
		RefreshToken: tokens.RefreshToken,
		TokenType: Bearer,
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
		respondProblem(c, http.StatusUnauthorized, model.ErrCodeUnauthorized, model.MsgUnauthorized)
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

func (h *AuthHandler) UpdateMe(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		respondProblem(c, http.StatusUnauthorized, model.ErrCodeUnauthorized, model.MsgUnauthorized)
		return
	}
	userID, ok := userIDVal.(int64)
	if !ok {
		respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		return
	}

	var req UpdateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Invalid request body")
		return
	}
	if req.FullName == nil || strings.TrimSpace(*req.FullName) == "" {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "full_name is required")
		return
	}

	var genderStr *string
	if req.Gender != nil {
		g := string(*req.Gender)
		genderStr = &g
	}

	userMe, err := h.authService.UpdateMe(c.Request.Context(), userID, *req.FullName, genderStr)
	if err != nil {
		switch {
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