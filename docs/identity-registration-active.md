# Identity — Inscription par email et activation immédiate

**Date :** 2026-11-25  
**Branche :** `feat/SCRUM-126-Contract`  
**Périmètre :** `services/identity`  
**Statut :** implémenté localement, à committer  
**Objectif :** débloquer le parcours `register → login` pour les tests, sans flux de vérification email.

---

## 1. Résumé

Avant ce changement, l’inscription :

- acceptait un email **ou** un téléphone ;
- créait systématiquement le compte en `pending_verification` ;
- rendait donc les nouveaux comptes incapables de se connecter, puisqu’aucun endpoint de vérification email n’existe.

Après ce changement :

- **l’email est obligatoire** ;
- le téléphone reste accepté, mais uniquement comme donnée de profil optionnelle ;
- le compte est créé directement en **`active`** ;
- l’utilisateur peut se connecter immédiatement après son inscription ;
- les comptes existants encore en `pending_verification` sont activés par une migration.

> Le statut `pending_verification` reste présent dans l’enum PostgreSQL pour compatibilité avec d’anciennes données, mais **plus aucun nouveau compte ne reçoit ce statut**.

---

## 2. Comportement avant / après

| Cas | Avant | Après |
|---|---|---|
| Inscription avec email | ✅ Acceptée | ✅ Acceptée |
| Inscription avec email + téléphone | ✅ Acceptée | ✅ Acceptée, téléphone optionnel |
| Inscription avec téléphone seul | ✅ Acceptée | ❌ `422 Email is required` |
| Statut après inscription | `pending_verification` | `active` |
| Connexion immédiate après inscription | ❌ `403 account_pending_verification` | ✅ `200` avec tokens |
| Vérification email | Inexistante | Toujours inexistante (hors périmètre) |
| Comptes existants en attente | Bloqués | Activés par la migration `0004` |

---

## 3. Contrat OpenAPI

Fichier : `services/identity/contracts/openapi.yaml`

### 3.1 `RegisterRequest`

Le schéma exige désormais explicitement l’email :

```yaml
RegisterRequest:
  type: object
  required: [fullName, email, password, userType]
  additionalProperties: false
  properties:
    fullName:
      type: string
      minLength: 1
      maxLength: 150
    gender:
      type: string
      enum: [male, female, other]
    email:
      type: string
      format: email
      maxLength: 254
      description: Login identifier. Required; the account is active immediately.
    phone:
      type: string
      maxLength: 30
      description: Optional profile data; not a login identifier.
    password:
      type: string
      format: password
      writeOnly: true
      minLength: 8
      maxLength: 72
    userType:
      type: string
      enum: [student, guest]
    subtypeAttributes:
      type: object
      additionalProperties: true
```

Le bloc `anyOf` qui rendait l’email ou le téléphone obligatoire a été supprimé.

### 3.2 Description de l’endpoint

La description de `POST /auth/register` précise maintenant :

- l’email est obligatoire ;
- le compte est actif immédiatement ;
- aucune vérification email n’est effectuée pour l’instant ;
- le téléphone n’est pas un identifiant de connexion.

---

## 4. Backend

### 4.1 Handler — validation de l’email

Fichier : `services/identity/api/internal/handler/auth.go`

```go
emailStr := strings.ToLower(strings.TrimSpace(string(req.Email)))
if emailStr == "" {
    respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Email is required")
    return
}
```

- L’ancien contrôle `hasEmail || hasPhone` a été supprimé.
- Une requête sans email est rejetée en `422`.
- L’email est normalisé en minuscules avant d’être transmis au service.
- Le champ `Phone` reste transmis s’il est présent, mais n’est plus requis.

Le mapping d’erreur suivant a également été ajouté :

```go
case errors.Is(err, model.ErrEmailRequired):
    respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Email is required")
```

### 4.2 Service — compte actif immédiatement

Fichier : `services/identity/api/internal/service/auth.go`

```go
newUser := model.User{
    FirstName:    firstName,
    LastName:     lastName,
    Email:        email,
    PhoneNumber:  phone,
    Gender:       input.Gender,
    PasswordHash: string(hash),
    Affiliation:  affiliation,
    Role:         role,
    // There is no email-verification flow yet: new accounts are usable
    // immediately so the register -> login path can be tested end to end.
    Status:       model.StatusActive,
    CampusID:     input.CampusID,
}
```

Un garde-fou côté service empêche également l’inscription sans email :

```go
// Email is the only login identifier. Enforce it here as well as in the
// HTTP handler so the rule cannot be bypassed by another caller.
if email == nil || *email == "" {
    return nil, model.ErrEmailRequired
}
```

### 4.3 Erreur dédiée

Fichier : `services/identity/api/internal/model/errors.go`

```go
ErrEmailRequired = errors.New("email is required")
```

### 4.4 Refactor de testabilité

Fichier : `services/identity/api/internal/service/auth.go`

`AuthService` dépend maintenant d’interfaces plutôt que des implémentations concrètes :

- `UserRepository`
- `SessionRepository`

Cela permet de tester le service sans PostgreSQL. `cmd/server` continue de passer les repositories concrets existants, sans changement de comportement.

### 4.5 Code généré

Fichier : `services/identity/api/internal/handler/generated.go`

Régénéré avec :

```bash
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 \
  -generate types,gin-server \
  -package handler \
  -o services/identity/api/internal/handler/generated.go \
  services/identity/contracts/openapi.yaml
```

Effets :

- suppression de l’union `RegisterRequest0` / `RegisterRequest1` ;
- `Email` devient un champ obligatoire non pointeur ;
- suppression de `userType: "worker"` du code généré, qui ne figurait plus dans le contrat.

---

## 5. Migration `0004`

### 5.1 Up

Fichier : `services/identity/api/migrations/0004_activate_registered_users.up.sql`

```sql
ALTER TABLE users
    ALTER COLUMN status SET DEFAULT 'active'::user_status;

UPDATE users
   SET status = 'active'::user_status
 WHERE status = 'pending_verification'::user_status;
```

Effets :

- le statut par défaut en base devient `active` ;
- tous les comptes encore en `pending_verification` sont activés ;
- les utilisateurs concernés peuvent se connecter sans intervention manuelle.

### 5.2 Down

Fichier : `services/identity/api/migrations/0004_activate_registered_users.down.sql`

```sql
ALTER TABLE users
    ALTER COLUMN status SET DEFAULT 'pending_verification'::user_status;
```

Le down restaure uniquement le défaut précédent. Il ne remet **pas** les comptes en `pending_verification` : la migration ne peut pas savoir quels comptes étaient déjà actifs avant le up, et reverrouiller tous les utilisateurs serait destructif.

### 5.3 Application

```bash
make migrate-up-identity
```

Ou directement dans le service :

```bash
cd services/identity/api
go run ./cmd/migrate up
```

---

## 6. Tests

### 6.1 Nouveau test de régression

Fichier : `services/identity/api/internal/service/auth_register_test.go`

Deux cas sont couverts :

1. `TestRegisterCreatesActiveUser`
   - vérifie que le statut de l’utilisateur créé est `active` ;
   - vérifie que le profil retourné est également `active` ;
   - vérifie que l’email créé est correct.

2. `TestRegisterRequiresEmail`
   - vérifie que `Register` échoue avec `ErrEmailRequired` si l’email est absent.

Exécution :

```bash
cd services/identity/api
go test ./...
```

Résultat observé :

```text
ok  github.com/transcandence/campusplusplus/services/identity/api/internal/service
```

### 6.2 Vérifications complémentaires

```bash
cd services/identity/api
go build ./...
go vet ./...
```

Les deux commandes passent.

Le code généré a été comparé à une régénération complète : `generated.go` est à jour.

---

## 7. Client TypeScript

Fichiers :

- `platform/ts/api-client/src/generated/identity.ts`
- `platform/ts/api-client/src/identity/identity.types.ts`

### 7.1 Régénération

```bash
./node_modules/.bin/openapi-typescript \
  services/identity/contracts/openapi.yaml \
  -o platform/ts/api-client/src/generated/identity.ts
```

Effets :

- `RegisterRequest` n’est plus une union contenant `unknown` ;
- `email` devient obligatoire ;
- `userType` ne contient plus `"worker"` ;
- le type est maintenant exploitable côté frontend.

### 7.2 Export du type

`identity.types.ts` exporte désormais :

```ts
export type RegisterRequest = components["schemas"]["RegisterRequest"];
```

### 7.3 Erreurs TypeScript préexistantes

`tsc` signale toujours deux erreurs **non liées à ce changement** :

```text
frontend/src/app/layout.tsx(20,50): error TS2304: Cannot find name 'LayoutProps'.
frontend/src/lib/auth/api.ts(22,13): error TS2554: Expected 1 arguments, but got 0.
```

Elles doivent être traitées séparément.

---

## 8. Documentation mise à jour

- `services/identity/README.md`
  - ajout de `POST /auth/register` dans le tableau des endpoints ;
  - mention du compte actif immédiat.

- `docs/identity-auth-spec.md`
  - `pending_verification` documenté comme valeur legacy ;
  - statut par défaut `active` ;
  - email et téléphone documentés conformément à la migration `0003`.

---

## 9. Liste des fichiers modifiés / ajoutés

| Fichier | Type | Description |
|---|---|---|
| `services/identity/contracts/openapi.yaml` | Modifié | Email obligatoire, description du compte actif, suppression du `anyOf`. |
| `services/identity/api/internal/handler/auth.go` | Modifié | Validation email obligatoire, mapping `ErrEmailRequired`. |
| `services/identity/api/internal/service/auth.go` | Modifié | Statut `active`, garde-fou email, interfaces de repositories. |
| `services/identity/api/internal/model/errors.go` | Modifié | Ajout de `ErrEmailRequired`. |
| `services/identity/api/internal/handler/generated.go` | Régénéré | Nouveau `RegisterRequest`, suppression de l’union. |
| `services/identity/api/migrations/0004_activate_registered_users.up.sql` | Ajouté | Activation des comptes en attente + défaut `active`. |
| `services/identity/api/migrations/0004_activate_registered_users.down.sql` | Ajouté | Restauration du défaut `pending_verification`. |
| `services/identity/api/internal/service/auth_register_test.go` | Ajouté | Tests de régression inscription active / email requis. |
| `platform/ts/api-client/src/generated/identity.ts` | Régénéré | Type `RegisterRequest` corrigé. |
| `platform/ts/api-client/src/identity/identity.types.ts` | Modifié | Export de `RegisterRequest`. |
| `services/identity/README.md` | Modifié | Documentation de `/auth/register`. |
| `docs/identity-auth-spec.md` | Modifié | Statuts et colonnes email/téléphone mis à jour. |
| `docs/identity-registration-active.md` | Ajouté | Ce document. |

---

## 10. Procédure de test manuel

### 10.1 Appliquer la migration

```bash
make migrate-up-identity
```

### 10.2 Inscrire un utilisateur

```bash
curl -i -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "fullName": "Test User",
    "email": "test@example.com",
    "password": "Password123",
    "userType": "student"
  }'
```

Réponse attendue :

- HTTP `201` ;
- `status: "active"` dans le profil retourné.

### 10.3 Se connecter immédiatement

```bash
curl -i -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Password123"
  }'
```

Réponse attendue :

- HTTP `200` ;
- `accessToken`, `refreshToken`, `expiresIn`, `tokenType`.

### 10.4 Vérifier le rejet du téléphone seul

```bash
curl -i -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "fullName": "Phone Only",
    "phone": "+212600000099",
    "password": "Password123",
    "userType": "student"
  }'
```

Réponse attendue :

- HTTP `422` ;
- message `Email is required`.

### 10.5 Vérifier l’activation des anciens comptes

```sql
SELECT id, email, status
FROM users
WHERE status = 'pending_verification';
```

Après la migration, cette requête doit retourner **0 ligne**.

---

## 11. Rollback

### 11.1 Rollback de la migration

```bash
make migrate-down-identity
```

Cela restaure le défaut `pending_verification` mais ne reverrouille pas les comptes déjà activés.

Pour reverrouiller explicitement des comptes (opération manuelle et destructive) :

```sql
UPDATE users
   SET status = 'pending_verification'
 WHERE email = 'test@example.com';
```

### 11.2 Rollback du code

Restaurer les fichiers modifiés et régénérer le code :

```bash
git checkout -- services/identity/contracts/openapi.yaml \
  services/identity/api/internal/handler/auth.go \
  services/identity/api/internal/service/auth.go \
  services/identity/api/internal/model/errors.go

go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 \
  -generate types,gin-server \
  -package handler \
  -o services/identity/api/internal/handler/generated.go \
  services/identity/contracts/openapi.yaml
```

---

## 12. Impacts et compatibilité

- **Clients API :** le champ `email` est désormais obligatoire à l’inscription. Les clients qui envoyaient uniquement `phone` recevront `422`.
- **Comptes existants :** ceux qui étaient en `pending_verification` deviennent `active`.
- **Base de données :** le défaut de `users.status` devient `active`.
- **Enum :** `pending_verification` reste disponible pour d’anciennes données ou des usages administratifs futurs.
- **Connexion par téléphone :** toujours non supportée. Le téléphone reste une donnée de profil.
- **Code généré :** Go et TypeScript doivent rester synchronisés avec `openapi.yaml`.

---

## 13. Limites connues et suites

Ce changement débloque les tests, mais ne remplace pas un vrai flux de vérification d’email :

- n’importe qui peut s’inscrire avec une adresse email qui ne lui appartient pas ;
- aucun rate limiting n’est actif sur `/auth/register` ;
- une inscription concurrente avec le même email peut encore produire un `500` au lieu d’un `409` (course entre `CheckEmailOrPhoneExists` et la contrainte `UNIQUE`) ;
- l’outbox `user.created` est alimentée, mais le worker n’est pas déployé ;
- le login reste limité à l’email.

Prochaines étapes recommandées :

1. ajouter le rate limiting sur register/login/refresh ;
2. mapper la violation `UNIQUE` PostgreSQL en `409` ;
3. implémenter un vrai flux de vérification email ;
4. déployer le worker outbox ;
5. ajouter des tests d’intégration HTTP + PostgreSQL.

---

## 14. Checklist PR

- [ ] Migration `0004` appliquée et testée sur une base contenant des comptes `pending_verification`.
- [ ] Inscription email → `201` puis login → `200`.
- [ ] Inscription téléphone seul → `422`.
- [ ] `go build ./...`, `go test ./...`, `go vet ./...` passent.
- [ ] `generated.go` et `identity.ts` sont à jour.
- [ ] Contrat OpenAPI et README cohérents.
- [ ] Aucune régression sur les comptes seedés.
