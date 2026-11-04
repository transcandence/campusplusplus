package main

import (
    "fmt"

    "github.com/transcandence/campusplusplus/platform/go/auth"
    "github.com/google/uuid"
)

func main() {
	fmt.Println("--- Étape 1 : Génération des clés RSA ---")
	if err := auth.GenerateKeys(); err != nil {
		fmt.Printf("Erreur lors de la génération des clés : %v\n", err)
		return
	}
	fmt.Println("Succès : 'private.pem' et 'public.pem' ont été créés.")

	fmt.Println("\n--- Étape 2 : Chargement de la clé privée ---")
	privateKey, err := auth.LoadPrivateKey("private.pem")
	if err != nil {
		fmt.Printf("Erreur chargement clé privée : %v\n", err)
		return
	}

	fmt.Println("\n--- Étape 3 : Génération du JWT ---")
    myuuid := uuid.New()
	tokenString, err := auth.GenerateAccessToken(42, myuuid, 3, 45, privateKey)
	if err != nil {
		fmt.Printf("Erreur génération token : %v\n", err)
		return
	}
	fmt.Printf("Token généré :\n%s\n", tokenString)

	fmt.Println("\n--- Étape 4 : Chargement de la clé publique ---")
	publicKey, err := auth.LoadPublicKey("public.pem")
	if err != nil {
		fmt.Printf("Erreur chargement clé publique : %v\n", err)
		return
	}

	fmt.Println("\n--- Étape 5 : Vérification du token ---")
	claims, err := auth.VerifyAccessToken(tokenString, publicKey)
	if err != nil {
		fmt.Printf("Erreur vérification token : %v\n", err)
		return
	}

	fmt.Println("Succès ! Le token est valide. Contenu des Claims :")
	fmt.Printf(" -> UserID   : %d\n", claims.UserID)
	fmt.Printf(" -> SessionID : %d\n", claims.SessionID)
	fmt.Printf(" -> FamilyID : %s\n", claims.FamilyID.String())
	fmt.Printf(" -> Issuer   : %s\n", claims.Issuer)
	fmt.Printf(" -> Expire   : %v\n", claims.ExpiresAt.Time)
}

/*
func main() {
    fmt.Println("=== 🚀 Démarrage du test du système d'authentification ===")

    privateKeyPath := os.Getenv("JWT_PRIVATE_KEY_PATH")
    if privateKeyPath == "" {
        privateKeyPath = "tests/private.pem"
    }

    publicKeyPath := os.Getenv("JWT_PUBLIC_KEY_PATH")
    if publicKeyPath == "" {
        publicKeyPath = "tests/public.pem"
    }

    privateKey, err := auth.LoadPrivateKey(privateKeyPath)
    if err != nil {
        log.Fatalf("❌ Erreur chargement clé privée: %v", err)
    }
    fmt.Println(" [OK] Clé privée chargée avec succès.")
    publicKey, err := auth.LoadPublicKey(publicKeyPath)
    if err != nil {
        log.Fatalf("❌ Erreur chargement clé publique: %v", err)
    }
    fmt.Println(" [OK] Clé publique chargée avec succès.")

    userID := 42
    role := "student"

    tokenString, err := auth.GenerateAccessToken(userID, role, privateKey)
    if err != nil {
        log.Fatalf("❌ Erreur génération token: %v", err)
    }
    fmt.Printf(" [OK] Token généré avec succès :\n      %s\n", tokenString)

    claims, err := auth.VerifyAccessToken(tokenString, publicKey)
    if err != nil {
        log.Fatalf("❌ Erreur vérification token valide: %v", err)
    }
    fmt.Println(" [OK] Token vérifié avec succès par le 'vigile' !")
    fmt.Printf("      -> Utilisateur ID (sub) : %d\n", claims.UserID)
    fmt.Printf("      -> Rôle : %s\n", claims.Role)
    fmt.Printf("      -> Expire à : %v\n", claims.ExpiresAt.Time)

    fakeTokenString := tokenString + "tricherie"
    _, err = auth.VerifyAccessToken(fakeTokenString, publicKey)
    if err != nil {
        fmt.Println(" [OK] Test de sécurité réussi : Le token falsifié a bien été rejeté !")
    } else {
        log.Fatal(" [ALERTE] Le token falsifié a été accepté !")
    }

    fmt.Println("\n=== ✨ Tout fonctionne à la perfection ! ===")
}
*/