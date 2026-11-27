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

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Invalid request body")
		return
	}
	fullName := strings.TrimSpace(req.FullName)
	if !validFullName(fullName) {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "fullName must be 1-150 characters, with each word at most 30 characters")
		return
	}
	emailStr, ok := normalizeEmail(string(req.Email))
	if !ok {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Email is required and must be valid")
		return
	}
	if req.Password == nil || *req.Password == "" {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Password is required")
		return
	}
	if len(*req.Password) < 8 || len(*req.Password) > 72 {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Password must be between 8 and 72 characters")
		return
	}
	var genderStr *string
	if req.Gender != nil {
		g := string(*req.Gender)
		if !validGender(g) {
			respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Gender must be one of: male, female, other")
			return
		}
		genderStr = &g
	}
	var phoneStr *string
	if req.Phone != nil {
		p, ok := normalizePhone(*req.Phone)
		if !ok {
			respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Phone must be 1-20 characters")
			return
		}
		phoneStr = &p
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
		FullName: fullName,
		Email:    &emailStr,
		Phone:    phoneStr,
		Password: req.Password,
		UserType: string(req.UserType),
		Gender:   genderStr,
		CampusID: campusID,
		// Affiliation: model.Affiliation,
	}
	result, err := h.authService.Register(c.Request.Context(), input)
	if err != nil {
		log.Printf("REGISTER ERROR: %+v", err)
		switch {
		case errors.Is(err, model.ErrUserAlreadyExists):
			respondProblem(c, http.StatusConflict, model.ErrCodeUserAlreadyExists, "An account with this email or phone already exists")
		case errors.Is(err, model.ErrEmailRequired):
			respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Email is required")
		case errors.Is(err, model.ErrInvalidInput):
			respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Invalid input")
		case errors.Is(err, model.ErrInvalidRole):
			respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeInvalidRole, model.MsgInvalidRole)
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
	if err := c.ShouldBindJSON(&req); err != nil {
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
		AccessToken:  tokens.AccessToken,
		ExpiresIn:    tokens.ExpiresIn,
		RefreshToken: tokens.RefreshToken,
		TokenType:    Bearer,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Invalid request body")
		return
	}
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if refreshToken == "" {
		respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Refresh token is required")
		return
	}

	if err := h.authService.Logout(c.Request.Context(), refreshToken); err != nil {
		// The contract makes logout idempotent: an unknown or already-revoked
		// token is a successful logout. Any other storage error is a real 500.
		if errors.Is(err, model.ErrSessionNotFound) {
			c.Status(http.StatusNoContent)
			return
		}
		respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		return
	}
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

func (h *AuthHandler) ListSessions(c *gin.Context) {
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

	sessions, err := h.authService.ListSessions(c.Request.Context(), userID)
	if err != nil {
		respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		return
	}

	currentSessionID := c.GetInt64("sessionID")
	response := make([]SessionSummary, 0, len(sessions))
	for _, session := range sessions {
		ipAddress := session.IPAddress
		response = append(response, SessionSummary{
			Id:         session.ID,
			CreatedAt:  session.CreatedAt,
			ExpiresAt:  session.ExpiresAt,
			LastUsedAt: session.LastUsedAt,
			IpAddress:  &ipAddress,
			UserAgent:  session.UserAgent,
			Current:    session.ID == currentSessionID,
		})
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, response)
}

func (h *AuthHandler) RevokeSession(c *gin.Context, sessionID int64) {
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

	revoked, err := h.authService.RevokeSession(c.Request.Context(), userID, sessionID)
	if err != nil {
		respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		return
	}
	if !revoked {
		respondProblem(c, http.StatusNotFound, model.ErrCodeSessionNotFound, "Session not found")
		return
	}

	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusNoContent)
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

	input := model.UpdateMeInput{}

	if req.FullName != nil {
		fullName := strings.TrimSpace(*req.FullName)
		if !validFullName(fullName) {
			respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "fullName must be 1-150 characters, with each word at most 30 characters")
			return
		}
		input.FullName = &fullName
	}

	if req.Gender != nil {
		gender := string(*req.Gender)
		if !validGender(gender) {
			respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Gender must be one of: male, female, other")
			return
		}
		input.Gender = &gender
	}

	if req.Email != nil {
		email, ok := normalizeEmail(string(*req.Email))
		if !ok {
			respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Email must be valid")
			return
		}
		input.Email = &email
	}

	if req.Phone != nil {
		phone, ok := normalizePhone(*req.Phone)
		if !ok {
			respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Phone must be 1-20 characters")
			return
		}
		input.Phone = &phone
	}

	userMe, err := h.authService.UpdateMe(c.Request.Context(), userID, input)
	if err != nil {
		switch {
		case errors.Is(err, model.ErrUserNotFound):
			respondProblem(c, http.StatusUnauthorized, model.ErrCodeUnauthorized, "User not found")
		case errors.Is(err, model.ErrUserAlreadyExists):
			respondProblem(c, http.StatusConflict, model.ErrCodeUserAlreadyExists, "An account with this email or phone already exists")
		case errors.Is(err, model.ErrInvalidInput):
			respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Invalid input")
		default:
			respondProblem(c, http.StatusInternalServerError, model.ErrCodeInternalError, model.MsgInternalError)
		}
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, userMe)
}
