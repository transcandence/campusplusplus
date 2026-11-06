package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/transcandence/campusplusplus/platform/go/auth"
)
 
func main() {
	fmt.Println("=== 1. Génération des clés RSA ===")
	err := auth.GenerateKeys()
	if err != nil {
		log.Fatalf("Erreur génération des clés : %v", err)
	}
	fmt.Println("-> Clés privées et publiques générées avec succès (private.pem / public.pem)")
	fmt.Println("\n=== 2. Chargement des clés ===")
	privateKey, err := auth.LoadPrivateKey("private.pem")
	if err != nil {
		log.Fatalf("Erreur chargement clé privée : %v", err)
	}
	publicKey, err := auth.LoadPublicKey("public.pem")
	if err != nil {
		log.Fatalf("Erreur chargement clé publique : %v", err)
	}
	fmt.Println("-> Clés chargées en mémoire avec succès.")
	fmt.Println("\n=== 3. Génération d'un Access Token (JWT) ===")
	userID := int64(42)
	familyID := uuid.New()
	campusID := int64(1)
	sessionID := int64(43)
	role := "student"
	accessTTL := 10 * time.Minute
	accessToken, err := auth.GenerateAccessToken(userID, sessionID, campusID, role, familyID, accessTTL, privateKey)
	if err != nil {
		log.Fatalf("Erreur génération JWT : %v", err)
	}
	fmt.Printf("-> JWT généré :\n%s\n", accessToken)
	fmt.Println("\n=== 4. Vérification et Décodage du JWT ===")
	claims, err := auth.VerifyAccessToken(accessToken, publicKey)
	if err != nil {
		log.Fatalf("Erreur vérification JWT : %v", err)
	}
	fmt.Printf("-> Token valide ! Claims extraits :\n")
	fmt.Printf("   - User ID  : %d\n", claims.UserID)
	fmt.Printf("   - Session ID: %d\n", claims.SessionID)
	fmt.Printf("   - Campus ID: %d\n", claims.CampusID)
	fmt.Printf("   - Role     : %s\n", claims.Role)
	fmt.Printf("   - Family ID: %s\n", claims.FamilyID)
	fmt.Printf("   - ExpiresAt: %v\n", claims.ExpiresAt.Time)
 
	fmt.Println("\n=== 5. Test des Opaque Refresh Tokens ===")
	rawRT, err := auth.GenerateOpaqueRefreshToken()
	if err != nil {
		log.Fatalf("Erreur génération refresh token : %v", err)
	}
	fmt.Printf("-> Refresh Token brut (à envoyer au client) : %s\n", rawRT)
 
	hashedRT := auth.HashRefreshToken(rawRT)
	fmt.Printf("-> Refresh Token haché (pour la colonne BYTEA) : %x\n", hashedRT)
	fmt.Printf("   (Taille du hash : %d octets)\n", len(hashedRT))
 
	fmt.Println("\n=== 6. Test du Hachage & Vérification de Mot de Passe (Bcrypt) ===")
	plainPassword := "MonSuperMotDePasse123!"
	 
	hashedPassword, err := auth.HashPassword(plainPassword)
	if err != nil {
		log.Fatalf("Erreur hachage mot de passe : %v", err)
	}
	fmt.Printf("-> Mot de passe en clair : %s\n", plainPassword)
	fmt.Printf("-> Hash bcrypt stocké    : %s\n", hashedPassword)
 
	isMatchCorrect := auth.CheckPassword(plainPassword, hashedPassword)
	fmt.Printf("-> Test avec le BON mot de passe   -> Résultat (doit être true) : %v\n", isMatchCorrect)
 
	isMatchWrong := auth.CheckPassword("MauvaisMotDePasse", hashedPassword)
	fmt.Printf("-> Test avec le MAUVAIS mot de passe -> Résultat (doit être false): %v\n", isMatchWrong)
}
