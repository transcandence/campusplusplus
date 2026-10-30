package handler

import(
	"database/sql"
	"encoding/json"
	"net/http"
	"github.com/transcandence/campusplusplus/platform/go/auth"
	"golang.org/x/crypto/bcrypt"
)

type RegisterRequest struct{
	Email string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct{
	Email string `json:"email"`
	Password string `json:"password"`
}

type User struct{
	ID int
	Email string
	PasswordHash string
	Role string
	Status string
}

func RegisterHandler(db *sql.DB) http.HandlerFunc{
	return func(w http.ResponseWriter, r *http.Request){
		if r.Method != http.MethodPost {
			http.Error(w, "Unauthorised method", http.StatusMethodNotAllowed)
			return
		} 
		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil{
			http.Error(w, "Invalid Data", http.StatusBadRequest)
			return
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil{
			http.Error(w, "Server internal error", http.StatusBadRequest)
			return
		}
		query := `INSERT INTO users (email, password_hash, role, status) VALUES ($1, $2, 'student', 'active') RETURNING id`
		var userID int
		err = db.QueryRow(query, req.Email, string(hashedPassword)).Scan(&userID)
		if err != nil{
			http.Error(w, "Error while creating email (mail already used ?)")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "User created successfuly",
            "user_id": userID,
		})
	}
}

func LogicHandler(db *sql.DB, privateKey interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request){
		if r.Method != http.MethodPost {
			http.Error(w, "Unauthorised method", http.StatusMethodNotAllowed)
			return
		} 
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil{
			http.Error(w, "Invalid Data", http.StatusBadRequest)
			return
		}
		var user User
		query := `SELECT id, email, password_hash, role, status FROM users WHERE email = $1`
		err := db.QueryRow(query, req.Email).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.Status)
		if err != nil {
			http.Error(w, "Email or password incorrect", http.StatusUnauthorized)
			return
		}
		if user.Status == "suspended" || user.Status == "archived" {
			http.Error(w, "Access refused : your account is suspended or archived", http.StatusForbidden)
			return
		}
		tokenString, err := auth.GenerateAccessToken(user.ID, user.Role, rsaPrivateKey)
		if err != nil {
			http.Error(w, "Error while generating token", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"access_token": tokenString,
		})
	}
}