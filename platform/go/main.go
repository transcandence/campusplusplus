package main

import (
    "fmt"
    "log"
    "os"

    "github.com/transcandence/campusplusplus/platform/go/auth"
)

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