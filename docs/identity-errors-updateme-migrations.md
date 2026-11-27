# Identity — Mappings d’erreurs, UpdateMe et corrections de migrations

**Date :** 2026-11-25  
**Branche :** `feat/SCRUM-126-Contract`  
**Périmètre :** `services/identity`  
**Statut :** implémenté localement, à committer  
**Objectif :** aligner les codes HTTP sur le contrat, finaliser `PUT /me`, et rendre les migrations `0002`/`0003` sûres.

---

## 1. Résumé

Cette étape corrige quatre zones :

1. **Mappings d’erreurs**
   - `account_suspended` sur login → `403 account_suspended` ;
   - violation d’unicité PostgreSQL `23505` → `409` ;
   - erreurs de validation PostgreSQL et validations applicatives → `422`.

2. **`PUT /me`**
   - tous les champs sont optionnels ;
   - `COALESCE` empêche d’effacer le genre ou les contacts omis ;
   - email et téléphone sont maintenant modifiables ;
   - collision email/téléphone → `409` ;
   - validation email, téléphone, genre et nom complet → `422`.

3. **Migration `0002`**
   - `campus_id` est ajouté nullable, backfillé, puis passé en `NOT NULL` ;
   - `TRUNCATE sessions` est supprimé : les sessions existantes sont conservées en hachant leur token ;
   - le down `0002` ne casse plus sur une table non vide.

4. **Down `0003`**
   - restaure `email NOT NULL` **et** `phone_number NOT NULL` ;
   - échoue explicitement si des comptes phone-only/email-only existent, au lieu de supprimer ou corrompre des données.

---

## 2. Mappings d’erreurs

### 2.1 Login : compte suspendu → 403

Fichier : `services/identity/api/internal/handler/auth.go`

Avant :

```go
case errors.Is(err, model.ErrAccountSuspended):
    respondProblem(c, http.StatusUnauthorized, model.ErrCodeInvalidRefresh, model.MsgInvalidRefresh)
```

Après :

```go
case errors.Is(err, model.ErrAccountSuspended):
    respondProblem(c, http.StatusForbidden, model.ErrCodeAccountSuspended, model.MsgAccountSuspended)
```

Cela respecte le contrat `openapi.yaml`, qui expose la distinction de statut **uniquement sur `/auth/login`**.

`/auth/refresh` conserve volontairement `401 invalid_refresh` pour tous les échecs, y compris un compte suspendu : c’est le comportement documenté par le contrat.

### 2.2 Violation d’unicité `23505` → 409

Fichier : `services/identity/api/internal/repository/user.go`

Un helper centralise la traduction des erreurs PostgreSQL :

```go
func mapUserWriteError(err error) error {
    var pgErr *pgconn.PgError
    if errors.As(err, &pgErr) {
        switch {
        case pgErr.Code == "23505":
            return model.ErrUserAlreadyExists
        case pgErr.Code == "23514" || strings.HasPrefix(pgErr.Code, "22"):
            return model.ErrInvalidInput
        }
    }
    return err
}
```

Utilisation :

- `CreateUserWithOutbox` → une inscription concurrente avec le même email/téléphone produit maintenant `409`, plus `500`.
- `UpdateMe` → un email/téléphone déjà utilisé produit `409`.

### 2.3 Erreurs de validation → 422

Nouvelle sentinelle :

```go
ErrInvalidInput = errors.New("invalid input")
```

Les codes PostgreSQL suivants sont traduits en `422` :

- `23514` : violation de `CHECK` ;
- toute la classe `22` : data exception (`22P02` valeur enum invalide, `22001` chaîne trop longue, etc.).

Les handlers `Register` et `UpdateMe` mappent `ErrInvalidInput` en :

```go
respondProblem(c, http.StatusUnprocessableEntity, model.ErrCodeValidationFailed, "Invalid input")
```

### 2.4 Correction du code d’erreur de rôle

Avant :

```go
ErrCodeInavlidRole = "invalid role"
```

Après :

```go
ErrCodeInvalidRole = "invalid_role"
```

Le statut reste `422`. La valeur `"invalid role"` avec un espace était difficile à consommer pour un client.

> Impact : un client qui testait littéralement `"invalid role"` doit utiliser `"invalid_role"`.

---

## 3. Finalisation de `UpdateMe`

### 3.1 Contrat

`UpdateMeRequest` est déjà entièrement optionnel dans `openapi.yaml` :

```yaml
UpdateMeRequest:
  type: object
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
    phone:
      type: string
      maxLength: 20
```

La limite téléphone est passée de `30` à `20` pour correspondre à la colonne `users.phone_number VARCHAR(20)`.

Le code Go et TypeScript générés ont été régénérés.

### 3.2 Handler

Fichier : `services/identity/api/internal/handler/auth.go`

`UpdateMe` ne exige plus `fullName`. Chaque champ est validé indépendamment s’il est présent :

| Champ | Validation | Erreur |
|---|---|---|
| `fullName` | 1–150 caractères, chaque mot ≤ 30 | `422` |
| `gender` | `male`, `female`, `other` | `422` |
| `email` | format email, ≤ 254 | `422` |
| `phone` | 1–20 caractères | `422` |

Un body vide `{}` est accepté : aucun champ n’est modifié (seul `updated_at` change en base), et le profil courant est retourné en `200`.

Les collisions sont mappées :

```go
case errors.Is(err, model.ErrUserAlreadyExists):
    respondProblem(c, http.StatusConflict, model.ErrCodeUserAlreadyExists, "An account with this email or phone already exists")
```

### 3.3 Service

Fichier : `services/identity/api/internal/service/auth.go`

Nouveau type d’entrée :

```go
type UpdateMeInput struct {
    FullName  *string
    FirstName *string
    LastName  *string
    Gender    *string
    Email     *string
    Phone     *string
}
```

Le service découpe `FullName` en `FirstName`/`LastName` uniquement s’il est fourni :

```go
if input.FullName != nil {
    firstName, lastName := splitFullName(*input.FullName)
    input.FirstName = &firstName
    input.LastName = &lastName
}
```

### 3.4 Repository — `COALESCE`

Fichier : `services/identity/api/internal/repository/user.go`

```sql
UPDATE users
   SET first_name   = COALESCE($2, first_name),
       last_name    = COALESCE($3, last_name),
       gender       = COALESCE($4::gender, gender),
       email        = COALESCE($5::citext, email),
       phone_number = COALESCE($6, phone_number),
       updated_at   = now()
 WHERE id = $1;
```

Conséquences :

- un champ absent (`nil`) conserve la valeur existante ;
- un `PUT` avec seulement `email` ne peut plus effacer le genre ;
- `email` et `phone` peuvent être modifiés ;
- une violation d’unicité est traduite en `409`.

> Limite assumée : envoyer `null` équivaut à omettre le champ. Il n’est donc pas encore possible d’effacer explicitement `gender`, `email` ou `phone` via `PUT /me`.

---

## 4. Corrections de migrations

### 4.1 `0002` — `campus_id`

Fichier : `services/identity/api/migrations/0002_login_reconcile.up.sql`

Avant :

```sql
ALTER TABLE users
    ADD COLUMN campus_id INTEGER NOT NULL
        REFERENCES campuses(id) ON DELETE RESTRICT;

UPDATE users SET campus_id = ... WHERE campus_id IS NULL;
```

`ADD COLUMN ... NOT NULL` échoue dès que `users` contient au moins une ligne.

Après :

```sql
ALTER TABLE users
    ADD COLUMN campus_id INTEGER
        REFERENCES campuses(id) ON DELETE RESTRICT;

UPDATE users
   SET campus_id = (SELECT id FROM campuses WHERE code = 'rabat')
 WHERE campus_id IS NULL;

ALTER TABLE users ALTER COLUMN campus_id SET NOT NULL;
```

### 4.2 `0002` — suppression du `TRUNCATE sessions`

Avant :

```sql
TRUNCATE sessions;

ALTER TABLE sessions DROP COLUMN token;
ALTER TABLE sessions ADD COLUMN token_hash BYTEA NOT NULL;
```

Après :

```sql
ALTER TABLE sessions ADD COLUMN token_hash BYTEA;

UPDATE sessions
   SET token_hash = sha256(convert_to(token, 'UTF8'));

ALTER TABLE sessions ALTER COLUMN token_hash SET NOT NULL;

ALTER TABLE sessions DROP COLUMN token;
ALTER TABLE sessions
    ADD CONSTRAINT sessions_token_hash_uniq UNIQUE (token_hash);
```

Hypothèse documentée : l’ancienne colonne `sessions.token` contenait le refresh token opaque brut, ce qui était le cas du code de login précédent.

- Les sessions existantes sont conservées.
- Si une ancienne valeur n’était pas le token brut, la session ne sera simplement plus utilisable et pourra être révoquée/expirée normalement.
- Aucun autre utilisateur ou session n’est supprimé.

### 4.3 `0002` — down

Fichier : `services/identity/api/migrations/0002_login_reconcile.down.sql`

Le down ne peut pas retrouver le token brut depuis un hash. Il restaure la forme de l’ancienne colonne en écrivant le hash hexadécimal :

```sql
ALTER TABLE sessions ADD COLUMN token TEXT;
UPDATE sessions SET token = encode(token_hash, 'hex');
ALTER TABLE sessions ALTER COLUMN token SET NOT NULL;
ALTER TABLE sessions
    ADD CONSTRAINT sessions_token_key UNIQUE (token);
ALTER TABLE sessions DROP COLUMN IF EXISTS token_hash;
```

Ces sessions ne fonctionneront plus avec l’ancienne application : elles doivent être révoquées après un rollback.

Le down ne recrée plus `UNIQUE(user_id)` : le schéma `0001` du dépôt ne définit pas cette contrainte, et l’application supporte plusieurs sessions par utilisateur.

### 4.4 `0003` — down

Fichier : `services/identity/api/migrations/0003_register_fix.down.sql`

Le down :

- supprime l’index et la table outbox ;
- supprime `users_contact_ck` ;
- vérifie qu’aucun utilisateur n’a `email IS NULL` ou `phone_number IS NULL` ;
- restaure `email NOT NULL` **et** `phone_number NOT NULL`.

Si des comptes phone-only ou email-only existent, la migration échoue avec un message explicite :

```text
Cannot roll back 0003: some users have NULL email or phone_number.
Backfill both columns (or remove those users) before running this rollback.
```

C’est volontaire : un rollback ne doit pas supprimer ou corrompre silencieusement des comptes.

---

## 5. Tests

### 5.1 Tests handler ajoutés

Fichier : `services/identity/api/internal/handler/auth_login_updateme_test.go`

| Test | Vérifie |
|---|---|
| `TestLoginSuspendedAccountReturns403` | Login d’un compte suspendu → `403 account_suspended` |
| `TestUpdateMeAcceptsPartialFields` | `{"email": ...}` sans `fullName` → `200` |
| `TestUpdateMeWithoutFullNameDoesNotRequireIt` | `{"gender":"female"}` → `200` |
| `TestUpdateMeRejectsInvalidGenderWith422` | Genre invalide → `422 validation_failed` |
| `TestUpdateMeDuplicateContactReturns409` | Contact déjà utilisé → `409 user_already_exists` |

### 5.2 Tests repository ajoutés

Fichier : `services/identity/api/internal/repository/user_error_test.go`

Vérifie la traduction :

- `23505` → `ErrUserAlreadyExists` ;
- `23514` → `ErrInvalidInput` ;
- `22P02` → `ErrInvalidInput` ;
- `22001` → `ErrInvalidInput` ;
- une erreur inconnue reste inchangée.

### 5.3 Tests migrations exécutés

Un test a été exécuté sur une **base PostgreSQL temporaire** (`postgres:17`), sans toucher à la base du projet.

Scénario validé :

1. `migrate up` sur base vide → version `4`, `dirty=false`.
2. `migrate down 3` → version `1`, `dirty=false`.
3. Insertion d’un utilisateur et d’une session avec token brut à la version `1`.
4. `migrate up` sur base **non vide** → version `4`, `dirty=false`.
5. Vérifications :
   - `users.status` a pour défaut `active` ;
   - `users.campus_id` est `NOT NULL` ;
   - le `token_hash` correspond bien à `sha256('raw-token-value')` ;
   - l’utilisateur existant est `active`.
6. `migrate down 3` avec des lignes existantes → version `1`, `dirty=false` ;
   - la colonne `token` restaurée contient 64 caractères hexadécimaux.
7. Insertion d’un utilisateur phone-only puis `migrate down 1` sur `0003` :
   - échec attendu avec le message explicite ;
   - aucune donnée supprimée.

### 5.4 Vérifications Go

```bash
cd services/identity/api
go build ./...
go test ./...
go vet ./...
```

Tous passent.

---

## 6. Fichiers modifiés / ajoutés

| Fichier | Type | Description |
|---|---|---|
| `services/identity/api/internal/handler/auth.go` | Modifié | `account_suspended` → 403, validations, `UpdateMe` complet, mappings 409/422. |
| `services/identity/api/internal/handler/validation.go` | Ajouté | Validation email, téléphone, genre, nom complet. |
| `services/identity/api/internal/handler/auth_login_updateme_test.go` | Ajouté | Tests login suspendu + `UpdateMe`. |
| `services/identity/api/internal/model/errors.go` | Modifié | `ErrInvalidInput`, correction `ErrCodeInvalidRole`. |
| `services/identity/api/internal/model/user.go` | Modifié | `UpdateMeInput`. |
| `services/identity/api/internal/repository/user.go` | Modifié | Mapping PostgreSQL, `UpdateMe` dynamique avec `COALESCE`. |
| `services/identity/api/internal/repository/user_error_test.go` | Ajouté | Tests du mapping `23505`/`22*`. |
| `services/identity/api/internal/service/auth.go` | Modifié | `UpdateMe` partiel via `UpdateMeInput`. |
| `services/identity/api/internal/service/auth_register_test.go` | Modifié | Fake repository aligné sur la nouvelle signature. |
| `services/identity/api/migrations/0002_login_reconcile.up.sql` | Modifié | `campus_id` nullable → backfill → NOT NULL ; suppression du TRUNCATE. |
| `services/identity/api/migrations/0002_login_reconcile.down.sql` | Modifié | Down compatible avec une table non vide, token hex. |
| `services/identity/api/migrations/0003_register_fix.down.sql` | Modifié | Guard NULL + restauration des deux NOT NULL. |
| `services/identity/contracts/openapi.yaml` | Modifié | `phone.maxLength` 30 → 20. |
| `services/identity/api/internal/handler/generated.go` | Régénéré | Contrat à jour. |
| `platform/ts/api-client/src/generated/identity.ts` | Régénéré | Contrat à jour. |
| `services/identity/README.md` | Modifié | Lien vers ce document. |
| `docs/identity-errors-updateme-migrations.md` | Ajouté | Ce document. |

---

## 7. Tests manuels

### 7.1 Login d’un compte suspendu

```bash
curl -i -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"suspended@example.com","password":"Password123"}'
```

Attendu :

```text
HTTP/1.1 403 Forbidden
Content-Type: application/problem+json
```

avec `code: account_suspended`.

### 7.2 Mise à jour partielle du profil

```bash
curl -i -X PUT http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{"email":"new-email@example.com"}'
```

Attendu : `200`, email mis à jour, genre et téléphone inchangés.

### 7.3 Collision d’email/téléphone

```bash
curl -i -X PUT http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{"phone":"+212600000001"}'
```

Attendu : `409 user_already_exists` si le numéro est déjà utilisé.

### 7.4 Validation invalide

```bash
curl -i -X PUT http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer <access-token>" \
  -H "Content-Type: application/json" \
  -d '{"gender":"robot"}'
```

Attendu : `422 validation_failed`.

### 7.5 Migrations

Sur une base de test uniquement :

```bash
cd services/identity/api
DATABASE_URL='postgres://.../campus_test?sslmode=disable' go run ./cmd/migrate up
DATABASE_URL='postgres://.../campus_test?sslmode=disable' go run ./cmd/migrate version
```

---

## 8. Rollback

### 8.1 Code

Restaurer les fichiers modifiés, puis régénérer le code :

```bash
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 \
  -generate types,gin-server \
  -package handler \
  -o services/identity/api/internal/handler/generated.go \
  services/identity/contracts/openapi.yaml

./node_modules/.bin/openapi-typescript \
  services/identity/contracts/openapi.yaml \
  -o platform/ts/api-client/src/generated/identity.ts
```

### 8.2 Migrations

- `0002` down : voir la section 4.3. Les sessions restaurées contiennent le hash hexadécimal et doivent être révoquées.
- `0003` down : échouera si des comptes phone-only/email-only existent. Il faut d’abord backfiller ou supprimer ces comptes.

### 8.3 Attention aux migrations déjà appliquées

Modifier `0002` corrige les **nouvelles** bases et les bases de test. Si une base existante a déjà appliqué `0002`, la migration ne sera pas rejouée. Si elle est dans un état `dirty` après un échec, il faut :

1. diagnostiquer l’état avec `migrate version` ;
2. réparer manuellement (colonnes/contraintes) ;
3. utiliser `migrate force <version>` avec précaution ;
4. relancer `migrate up`.

Ne jamais utiliser `force` sans avoir vérifié le schéma réel.

---

## 9. Limites restantes

- Le rate limiting n’est toujours pas implémenté.
- La vérification email n’existe toujours pas.
- `PUT /me` ne permet pas d’effacer explicitement `gender`/`email`/`phone` avec `null`.
- Les access tokens restent valides jusqu’à leur expiration (10 minutes) après un changement de statut.
- Le worker outbox n’est toujours pas déployé.
- `UpdateMe` ne gère pas de contrôle de concurrence (deux mises à jour simultanées : la dernière gagne).

---

## 10. Checklist PR

- [ ] Login compte suspendu → `403 account_suspended`.
- [ ] Inscription en doublon → `409`.
- [ ] `PUT /me` avec un seul champ → `200`.
- [ ] `PUT /me` sans `fullName` → `200`.
- [ ] Genre invalide → `422`.
- [ ] Email/téléphone déjà utilisé → `409`.
- [ ] `0002` s’applique sur une base contenant des utilisateurs et des sessions.
- [ ] Aucun `TRUNCATE sessions` dans `0002`.
- [ ] `0003` down échoue proprement si des NULL existent.
- [ ] `go build ./...`, `go test ./...`, `go vet ./...` passent.
- [ ] Migrations testées sur une base temporaire, pas seulement en local.
