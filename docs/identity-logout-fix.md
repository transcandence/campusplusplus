# Identity — Correction du logout

**Date :** 2026-11-25  
**Branche :** `feat/SCRUM-126-Contract`  
**Périmètre :** `services/identity`  
**Statut :** implémenté localement, à committer  
**Objectif :** rendre `POST /auth/logout` fiable : body obligatoire, erreurs propagées, `204` uniquement quand la session est réellement révoquée (ou déjà inconnue).

---

## 1. Résumé

Le handler `Logout` précédent avait trois problèmes :

```go
func (h *AuthHandler) Logout(c *gin.Context) {
    var req LogoutRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.Status(http.StatusNoContent)
        return
    }
    _ = h.authService.Logout(c.Request.Context(), req.RefreshToken)
    c.Status(http.StatusNoContent)
}
```

1. **Body invalide ou absent** → `204 No Content`, alors qu’aucune session n’avait été révoquée.
2. **Erreur de stockage ignorée** (`_ =`) → `204` même si la base était indisponible.
3. **Token vide** → appel au service avec une chaîne vide, puis `204`.

Après la correction :

- le body JSON est obligatoire ;
- `refreshToken` doit être non vide ;
- une erreur de lookup ou de révocation remonte en `500` ;
- `204` n’est renvoyé que si la révocation a réussi, ou si la session est déjà inconnue/révoquée (comportement idempotent du contrat).

---

## 2. Comportement avant / après

| Cas | Avant | Après |
|---|---|---|
| Body absent | `204` sans révocation | `422 Invalid request body` |
| JSON malformé | `204` sans révocation | `422 Invalid request body` |
| `refreshToken` vide | `204` sans révocation | `422 Refresh token is required` |
| Token inconnu | `204` | `204` (idempotent) |
| Token déjà révoqué | `204` | `204` (idempotent) |
| Session valide | `204` | `204`, session révoquée |
| Erreur DB au lookup | `204` mensonger | `500 internal_error` |
| Erreur DB à la révocation | `204` mensonger | `500 internal_error` |

---

## 3. Contrat OpenAPI

Fichier : `services/identity/contracts/openapi.yaml`

Le contrat imposait déjà :

- `requestBody.required: true` ;
- `refreshToken` obligatoire (`minLength: 1`) ;
- `204` idempotent pour un token valide, déjà révoqué ou inconnu ;
- `422` pour une validation échouée.

La correction aligne donc l’implémentation sur le contrat existant. Aucun changement de schéma n’a été nécessaire.

Réponses attendues :

| Statut | Signification |
|---|---|
| `204` | Session révoquée, déjà révoquée, ou inconnue |
| `422` | Body absent, malformé, ou `refreshToken` vide |
| `500` | Erreur de stockage (base indisponible, etc.) |
| `429` | Rate limiting (non implémenté à ce jour) |

---

## 4. Implémentation

### 4.1 Handler

Fichier : `services/identity/api/internal/handler/auth.go`

```go
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
```

Points clés :

- `Cache-Control: no-store` sur toutes les réponses de logout.
- `ShouldBindJSON` en erreur → `422`.
- `refreshToken` trimé puis vérifié non vide → `422`.
- Le service n’est plus appelé avec un token vide.
- Les erreurs du service ne sont plus ignorées.
- `ErrSessionNotFound` reste un `204` idempotent.
- Toute autre erreur devient un `500` Problem Details.

### 4.2 Service

Fichier : `services/identity/api/internal/service/auth.go`

Aucun changement de logique nécessaire : le service propagait déjà correctement les erreurs.

```go
func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
    tokenHash := auth.HashRefreshToken(rawRefreshToken)
    session, err := s.sessionRepo.FindByTokenHash(ctx, tokenHash)
    if err != nil {
        if errors.Is(err, model.ErrSessionNotFound) {
            return nil
        }
        return fmt.Errorf("finding session for logout: %w", err)
    }
    if err := s.sessionRepo.RevokeByID(ctx, session.ID); err != nil {
        return fmt.Errorf("revoking session: %w", err)
    }
    return nil
}
```

Le service reste idempotent : un token inconnu retourne `nil`, ce qui produit un `204`.

### 4.3 Repository

Fichier : `services/identity/api/internal/repository/session.go`

Correction d’une faute de frappe dans le message d’erreur :

```go
return fmt.Errorf("session repo revoke by id: %w", err)
```

`RevokeByID` conserve sa condition :

```sql
WHERE id = $1
  AND revoked_at IS NULL
```

Une session déjà révoquée ne produit donc aucune ligne modifiée, mais l’opération reste un succès idempotent.

---

## 5. Tests ajoutés

Fichier : `services/identity/api/internal/handler/auth_logout_test.go`

Le test monte un vrai handler Gin avec un `AuthService` réel et un repository de sessions en mémoire.

| Test | Vérifie |
|---|---|
| `TestLogoutRequiresBody` | Body absent → `422 validation_failed` |
| `TestLogoutRequiresRefreshToken` | `refreshToken: ""` → `422 Refresh token is required` |
| `TestLogoutUnknownSessionIsIdempotent` | Token inconnu → `204` + `Cache-Control: no-store` |
| `TestLogoutRevokesSession` | Session valide → `204` et `RevokeByID` appelé avec le bon ID |
| `TestLogoutPropagatesLookupError` | Erreur DB au lookup → `500 internal_error` |
| `TestLogoutPropagatesRevokeError` | Erreur DB à la révocation → `500 internal_error` |

Exécution :

```bash
cd services/identity/api
go test ./...
```

Résultat observé :

```text
ok  github.com/transcandence/campusplusplus/services/identity/api/internal/handler
ok  github.com/transcandence/campusplusplus/services/identity/api/internal/service
```

Vérifications complémentaires :

```bash
go build ./...
go vet ./...
```

Les deux commandes passent.

---

## 6. Documentation mise à jour

- `services/identity/README.md`
  - ajout d’un lien vers ce document ;
  - le tableau des endpoints indique déjà `logout` idempotent en `204`.

- `docs/identity-auth-spec.md`
  - section « Single-Device Logout » précisée :
    - body JSON obligatoire ;
    - `422` si body manquant/vide/invalide ;
    - `500` en cas d’erreur de stockage ;
    - `204` uniquement pour un succès réel ou idempotent.

- `docs/identity-logout-fix.md`
  - ce document.

---

## 7. Liste des fichiers modifiés / ajoutés

| Fichier | Type | Description |
|---|---|---|
| `services/identity/api/internal/handler/auth.go` | Modifié | Body obligatoire, token non vide, erreurs propagées, `Cache-Control`. |
| `services/identity/api/internal/repository/session.go` | Modifié | Correction du message d’erreur `RevokeByID`. |
| `services/identity/api/internal/handler/auth_logout_test.go` | Ajouté | 6 tests HTTP du logout. |
| `services/identity/README.md` | Modifié | Lien vers la documentation du correctif. |
| `docs/identity-auth-spec.md` | Modifié | Comportement logout précisé. |
| `docs/identity-logout-fix.md` | Ajouté | Ce document. |

Aucun changement de contrat OpenAPI n’a été nécessaire : le code est désormais conforme au contrat existant.

---

## 8. Test manuel

### 8.1 Logout avec un token valide

```bash
curl -i -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Content-Type: application/json" \
  -d '{"refreshToken":"<refresh-token>"}'
```

Réponse attendue :

```text
HTTP/1.1 204 No Content
Cache-Control: no-store
```

### 8.2 Logout sans body

```bash
curl -i -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Content-Type: application/json"
```

Réponse attendue :

```text
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/problem+json
```

Avec un code `validation_failed`.

### 8.3 Logout avec token vide

```bash
curl -i -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Content-Type: application/json" \
  -d '{"refreshToken":""}'
```

Réponse attendue : `422`, détail `Refresh token is required`.

### 8.4 Logout avec token inconnu

```bash
curl -i -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Content-Type: application/json" \
  -d '{"refreshToken":"unknown-token"}'
```

Réponse attendue : `204`, comportement idempotent.

---

## 9. Rollback

### 9.1 Code

Restaurer les fichiers :

```bash
git checkout -- services/identity/api/internal/handler/auth.go \
  services/identity/api/internal/repository/session.go
rm services/identity/api/internal/handler/auth_logout_test.go
```

### 9.2 Documentation

Retirer les ajouts dans `services/identity/README.md` et
`docs/identity-auth-spec.md`, puis supprimer ce document.

Aucune migration n’est associée à ce correctif.

---

## 10. Impacts et limites

- **Clients existants** : ceux qui appelaient `/auth/logout` sans body recevaient `204` ; ils recevront désormais `422`. C’est le comportement prévu par le contrat.
- **Sécurité** : une erreur de base ne produit plus un faux succès. Le refresh token reste donc valide si la révocation échoue, et le client reçoit un `500` explicite.
- **Access token** : comme avant, révoquer la session ne révoque pas immédiatement l’access token JWT déjà émis. Il reste valide jusqu’à son expiration (10 minutes). C’est un choix documenté du modèle sans état.
- **Rate limiting** : `/auth/logout` n’est toujours pas limité en débit.
- **Rotation** : un logout ne révoque que la session courante ; `logout-all` reste nécessaire pour toutes les sessions.

---

## 11. Checklist PR

- [ ] Logout avec body valide → `204` et session révoquée en base.
- [ ] Logout sans body → `422`.
- [ ] Logout avec `refreshToken` vide → `422`.
- [ ] Logout avec token inconnu → `204`.
- [ ] Erreur DB simulée → `500`, pas `204`.
- [ ] `go build ./...`, `go test ./...`, `go vet ./...` passent.
- [ ] Aucun changement de contrat OpenAPI nécessaire.
