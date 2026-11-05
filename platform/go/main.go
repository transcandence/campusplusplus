package main

import (
	"fmt"
	"log"
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
	userID := 42
	familyID := uuid.New()
	campusID := 1
	sessionID := 43
	accessToken, err := auth.GenerateAccessToken(userID, familyID, campusID, sessionID, privateKey)
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
	fmt.Printf("   - Family ID: %s\n", claims.FamilyID)
	fmt.Printf("   - Campus ID: %d\n", claims.CampusID)
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

// package main
// 
// import (
	// "context"
	// "database/sql"
	// "fmt"
	// "log"
	// "time"
// 
	// "github.com/google/uuid"
	// _ "github.com/lib/pq" // Driver PostgreSQL (pense à faire go get github.com/lib/pq si ce n'est pas fait)
	// "github.com/transcandence/campusplusplus/platform/go/auth"
// )
// 
// func main() {
	// fmt.Println("=== 1. Génération et Chargement des clés RSA ==kg ===")
	// _ = auth.GenerateKeys() // Ignore l'erreur si elles existent déjà
	// privateKey, err := auth.LoadPrivateKey("private.pem")
	// if err != nil {
		// log.Fatalf("Erreur chargement clé privée : %v", err)
	// }
// 
	// fmt.Println("\n=== 2. Connexion à la Base de Données PostgreSQL ===")
	// Adapte ton DSN (host, port, user, password, dbname, sslmode) selon ta config
	// connStr := "postgres://postgres:postgres@localhost:5431/campusplusplus?sslmode=disable"
	// db, err := sql.Open("postgres", connStr)
	// if err != nil {
		// log.Fatalf("Erreur ouverture connexion SQL : %v", err)
	// }
	// defer db.Close()
// 
	// Test de ping pour valider que la bdd répond
	// ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	// defer cancel()
	// if err := db.PingContext(ctx); err != nil {
		// log.Fatalf("Impossible de se connecter à PostgreSQL (vérifie ton container Docker) : %v", err)
	// }
	// fmt.Println("-> Connecté à PostgreSQL avec succès !")
// 
	// fmt.Println("\n=== 3. Test du Cycle de Refresh Token & Révocation ===")
	// 
	// A. Génération d'un faux refresh token brut pour le test
	// rawRT, err := auth.GenerateOpaqueRefreshToken()
	// if err != nil {
		// log.Fatalf("Erreur génération opaque token : %v", err)
	// }
	// 
	// Pour tester la fonction RefreshSession, il faudrait d'abord qu'une session existe en BDD avec ce hash.
	// Tu peux insérer une session de test manuellement en Go pour l'exercice :
	// hashedRT := auth.HashRefreshToken(rawRT)
	// familyID := uuid.New()
	// userID := 1 // Assure-toi d'avoir un user avec l'ID 1 et un campus valide en BDD
// 
	// fmt.Printf("-> Insertion d'une session de test en BDD (FamilyID: %s)...\n", familyID)
	// _, err = db.Exec(
		// `INSERT INTO sessions (user_id, token_hash, ip_address, user_agent, expires_at, family_id) 
		//  VALUES ($1, $2, '127.0.0.1', 'Test-Script', NOW() + INTERVAL '30 days', $3)`,
		// userID, hashedRT, familyID,
	// )
	// if err != nil {
		// log.Fatalf("Erreur insertion session test (vérifie que l'user_id 1 existe) : %v", err)
	// }
// 
	// fmt.Println("-> Session insérée. Testons maintenant la fonction RefreshSession...")
// 
	// B. Appel de ta fonction RefreshSession (Rotation normale)
	// newAccessToken, newRawRT, err := auth.RefreshSession(context.Background(), db, rawRT, privateKey)
	// if err != nil {
		// log.Fatalf("Erreur lors du RefreshSession : %v", err)
	// }
// 
	// fmt.Println("-> Succès ! RefreshSession a fonctionné :")
	// fmt.Printf("   - Nouveau JWT : %s...\n", newAccessToken[:30])
	// fmt.Printf("   - Nouveau Refresh Token : %s\n", newRawRT)
// 
	// fmt.Println("\n=== 4. Test de Détection de Vol (Reuse Attack) ===")
	// fmt.Println("-> Tentative de réutiliser l'ANCIEN refresh token (qui vient d'être roté)...")
// 
	// C. Tentative de réutiliser l'ancien token (doit déclencher l'alerte de sécurité et révoquer la famille)
	// _, _, err = auth.RefreshSession(context.Background(), db, rawRT, privateKey)
	// if err != nil {
		// fmt.Printf("-> Comportement attendu capturé avec succès : %v\n", err)
	// } else {
		// fmt.Println("-> ERREUR DE SÉCURITÉ : Le vieux token a été accepté alors qu'il aurait dû être rejeté !")
	// }
// }