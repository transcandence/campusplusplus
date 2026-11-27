# Identity — Cycle de vie des sessions

**Date :** 2026-11-25  
**Branche :** `feat/SCRUM-126-Contract`  
**Périmètre :** `services/identity` (backend)  
**Statut :** implémenté localement, à committer  
**Objectif :** rendre les sessions observables, limitées, nettoyées et révocables individuellement.

---

## 1. Résumé

Cinq problèmes de cycle de vie sont traités :

| Problème | Avant | Après |
|---|---|---|
| Erreur `RevokeFamily` ignorée | Le token volé restait utilisable si la DB échouait | L’erreur remonte, réponse `500`, la menace n’est pas masquée |
| `last_used_at` non renseigné | `NULL` sur la nouvelle session après login/refresh | Renseigné au login et à chaque rotation |
| Pas de GC | Les sessions expirées/révoquées s’accumulaient | Nettoyage périodique par le worker |
| Pas de limite de sessions | Un utilisateur pouvait créer un nombre illimité de sessions | Limite configurable, plus anciennes révoquées |
| Pas de listing/révocation | Impossible de voir ou tuer une session | `GET /sessions` et `DELETE /sessions/{id}` |

---

## 2. Propagation de `RevokeFamily`

Fichier : `services/identity/api/internal/service/auth.go`

### Avant

```go
if session.RevokedAt != nil && session.RotatedAt != nil {
    _ = s.sessionRepo.RevokeFamily(ctx, session.FamilyID)
    return nil, model.ErrInvalidRefresh
}
```

### Après

```go
if session.RevokedAt != nil && session.RotatedAt != nil {
    // Reuse of a rotated token means the family is compromised. If we
    // cannot revoke it, surface a 500 instead of pretending the threat
    // was handled.
    if err := s.sessionRepo.RevokeFamily(ctx, session.FamilyID); err != nil {
        return nil, fmt.Errorf("revoking reused session family: %w", err)
    }
    return nil, model.ErrInvalidRefresh
}
```

Comportement :

- si la révocation réussit → `401 invalid_refresh` ;
- si la DB échoue → `500 internal_error` ;
- le client ne reçoit pas un faux `401` alors que la famille est encore active.

Test ajouté : `TestRefreshReusePropagatesFamilyRevokeError`.

---

## 3. `last_used_at`

Fichier : `services/identity/api/internal/repository/session.go`

### Avant

`Create` et `Rotate` inséraient la session sans `last_used_at` :

```sql
INSERT INTO sessions (user_id, token_hash, ip_address, user_agent, expires_at, family_id)
VALUES ($1, $2, $3, $4, $5, $6)
```

### Après

Requête commune :

```sql
INSERT INTO sessions (
    user_id, token_hash, ip_address, user_agent,
    expires_at, family_id, last_used_at
)
VALUES ($1, $2, $3, $4, $5, $6, now())
RETURNING id, created_at, updated_at, last_used_at;
```

Utilisée par :

- `Create` / `CreateWithLimit` (login) ;
- `Rotate` (refresh), qui met aussi à jour l’ancienne session :

```sql
UPDATE sessions
   SET revoked_at = now(),
       rotated_at = now(),
       last_used_at = now()
 WHERE id = $1
   AND revoked_at IS NULL;
```

Résultat :

- `last_used_at` est non nul dès le login ;
- il est mis à jour sur l’ancienne et la nouvelle session lors d’un refresh ;
- `GET /sessions` peut l’afficher de façon fiable.

---

## 4. GC des sessions expirées/révoquées

### 4.1 Migration `0005`

Fichier : `services/identity/api/migrations/0005_session_management.up.sql`

```sql
ALTER TABLE sessions
    DROP CONSTRAINT IF EXISTS sessions_replaced_by_fkey;

ALTER TABLE sessions
    ADD CONSTRAINT sessions_replaced_by_fkey
    FOREIGN KEY (replaced_by) REFERENCES sessions(id)
    ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_sessions_user_created_active
    ON sessions (user_id, created_at DESC, id DESC)
    WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_sessions_expires_at_all
    ON sessions (expires_at);

CREATE INDEX IF NOT EXISTS idx_sessions_revoked_at
    ON sessions (revoked_at)
    WHERE revoked_at IS NOT NULL;
```

Pourquoi `ON DELETE SET NULL` :

- `replaced_by` référence une autre ligne de `sessions` ;
- sans cette règle, supprimer une vieille session encore référencée violerait la FK et bloquerait le GC.

Down : `0005_session_management.down.sql` restaure la FK sans cascade et supprime les index.

### 4.2 Méthode de nettoyage

Fichier : `services/identity/api/internal/repository/session.go`

```go
func (r *SessionRepository) DeleteStale(ctx context.Context, cutoff time.Time, limit int) (int64, error)
```

Requête :

```sql
DELETE FROM sessions
 WHERE id IN (
     SELECT id
       FROM sessions
      WHERE expires_at < $1
         OR (revoked_at IS NOT NULL AND revoked_at < $1)
      ORDER BY expires_at
      LIMIT $2
 );
```

Traitement par lots de 1000 pour éviter une transaction géante.

### 4.3 Worker

Fichier : `services/identity/api/cmd/worker/main.go`

Le worker existant exécute maintenant deux tâches :

1. outbox toutes les 2 secondes ;
2. nettoyage des sessions selon `SESSION_CLEANUP_INTERVAL`.

```go
cleanupTicker := time.NewTicker(cfg.SessionCleanupInterval)

case <-cleanupTicker.C:
    if err := cleanupSessions(ctx, sessionRepo, cfg.SessionRetention, logger); err != nil {
        logger.Error("session cleanup error", "error", err)
    }
```

`cleanupSessions` boucle par lots jusqu’à ce qu’il reste moins de 1000 lignes.

### 4.4 Déploiement

- `services/identity/api/Dockerfile` compile désormais `./cmd/worker` vers `/out/worker`.
- `infra/compose/docker-compose.yml` ajoute le service `identity-worker` :

```yaml
identity-worker:
  image: campus_identity:local
  command: ["./worker"]
  depends_on:
    identity-migrate:
      condition: service_completed_successfully
  restart: unless-stopped
  profiles: [identity, all]
```

Sans ce service, le GC et l’outbox ne tournent pas.

---

## 5. Limite de sessions actives

### 5.1 Configuration

```env
MAX_ACTIVE_SESSIONS=10
```

Validation : `MAX_ACTIVE_SESSIONS >= 1`.

### 5.2 Implémentation

Fichier : `services/identity/api/internal/repository/session.go`

Nouvelle méthode :

```go
func (r *SessionRepository) CreateWithLimit(ctx context.Context, s *model.Session, maxActive int) error
```

Fonctionnement transactionnel :

1. `pg_advisory_xact_lock(userID)` pour sérialiser les logins concurrents du même utilisateur ;
2. insertion de la nouvelle session ;
3. calcul du rang des sessions actives :

```sql
WITH ranked AS (
    SELECT id,
           row_number() OVER (
               PARTITION BY user_id
               ORDER BY created_at DESC, id DESC
           ) AS rn
      FROM sessions
     WHERE user_id = $1
       AND revoked_at IS NULL
)
UPDATE sessions
   SET revoked_at = now(),
       updated_at = now()
 WHERE id IN (SELECT id FROM ranked WHERE rn > $2);
```

Les `maxActive` sessions les plus récentes sont conservées, les plus anciennes sont révoquées.

Le login utilise désormais `CreateWithLimit` au lieu de `Create`.

---

## 6. Endpoints de listing / révocation

### 6.1 Contrat OpenAPI

Ajouts :

- `GET /sessions`
- `DELETE /sessions/{sessionId}`

Schéma :

```yaml
SessionSummary:
  type: object
  required: [id, createdAt, expiresAt, current]
  properties:
    id: { type: integer, format: int64 }
    createdAt: { type: string, format: date-time }
    expiresAt: { type: string, format: date-time }
    lastUsedAt: { type: string, format: date-time, nullable: true }
    ipAddress: { type: string }
    userAgent: { type: string, nullable: true }
    current: { type: boolean }
```

Aucune donnée sensible (`token_hash`, `family_id`, `replaced_by`) n’est exposée.

### 6.2 Repository

- `ListActiveByUserID(ctx, userID)` : sessions non révoquées et non expirées, triées par création décroissante.
- `RevokeByIDForUser(ctx, userID, sessionID)` : révoque uniquement si la session appartient à l’utilisateur.

```sql
UPDATE sessions
   SET revoked_at = now(),
       updated_at = now()
 WHERE id = $1
   AND user_id = $2
   AND revoked_at IS NULL;
```

Retourne `false` si la session n’existe pas, est déjà révoquée, ou appartient à un autre utilisateur.

### 6.3 Handler

Fichier : `services/identity/api/internal/handler/auth.go`

- `ListSessions` :
  - lit `userID` et `sessionID` depuis le contexte JWT ;
  - marque `current: true` pour la session du token courant ;
  - `Cache-Control: no-store` ;
  - `200` avec un tableau.
- `RevokeSession` :
  - `204` si révoquée ;
  - `404 session_not_found` sinon ;
  - `500` sur erreur DB.

### 6.4 Accès

Les deux endpoints sont protégés par `BearerAuthMiddleware` via `security: bearerAuth`.

> Le routage gateway de `/sessions` reste une étape d’intégration séparée : le backend Identity les sert directement, mais le gateway doit encore être configuré pour les exposer.

---

## 7. Configuration ajoutée

Fichiers :

- `services/identity/api/internal/config/config.go`
- `services/identity/api/.env.example`
- `services/identity/api/.env.docker.example`

| Variable | Défaut | Rôle |
|---|---|---|
| `MAX_ACTIVE_SESSIONS` | `10` | Nombre maximum de sessions actives par utilisateur |
| `SESSION_RETENTION` | `720h` | Durée de conservation avant GC |
| `SESSION_CLEANUP_INTERVAL` | `1h` | Fréquence du nettoyage dans le worker |

Validation :

- `MAX_ACTIVE_SESSIONS >= 1` ;
- `SESSION_RETENTION > 0` ;
- `SESSION_CLEANUP_INTERVAL > 0`.

---

## 8. Tests

### 8.1 Tests unitaires

- `TestRefreshReusePropagatesFamilyRevokeError` : une erreur de `RevokeFamily` remonte bien.
- `TestListSessionsMarksCurrentSession`.
- `TestListSessionsRepositoryErrorReturns500`.
- `TestRevokeSessionSuccess`.
- `TestRevokeSessionNotOwnedReturns404`.
- `TestRevokeSessionRepositoryErrorReturns500`.

### 8.2 Test d’intégration PostgreSQL

Fichier : `services/identity/api/internal/repository/session_integration_test.go`

Gated par `TEST_DATABASE_URL`. Il vérifie :

- création de sessions avec `last_used_at` renseigné ;
- limite `maxActive=2` : seulement 2 sessions actives après 3 logins ;
- listing des sessions actives ;
- révocation de sa propre session ;
- refus de révoquer la session d’un autre utilisateur ;
- GC d’une session ancienne (`DeleteStale`).

Exécution observée sur une base temporaire PostgreSQL 17 :

```text
--- PASS: TestSessionLifecycleIntegration
```

### 8.3 Migration `0005`

Testée sur base temporaire :

- `up` → version 5, `dirty=false` ;
- contrainte obtenue : `FOREIGN KEY (replaced_by) REFERENCES sessions(id) ON DELETE SET NULL` ;
- index créés : `idx_sessions_user_created_active`, `idx_sessions_expires_at_all`, `idx_sessions_revoked_at` ;
- `down 1` → version 4, FK sans `ON DELETE SET NULL` ;
- `up` → version 5 de nouveau.

### 8.4 Vérifications Go

```bash
cd services/identity/api
go build ./...
go test ./...
go vet ./...
```

Tous passent.

---

## 9. Tests manuels

### 9.1 Lister ses sessions

> En attendant le routage gateway de `/sessions`, ces appels se font
> directement sur Identity. Avec le compose, Identity est exposé sur `4040` ;
> en lancement local, utilisez `http://localhost:8080`.

```bash
curl -i http://localhost:4040/sessions \
  -H "Authorization: Bearer <access-token>"
```

Attendu : `200`, tableau avec `current: true` sur la session courante.

### 9.2 Révoquer une session

```bash
curl -i -X DELETE http://localhost:4040/sessions/42 \
  -H "Authorization: Bearer <access-token>"
```

Attendu :

- `204` si la session appartient à l’utilisateur ;
- `404 session_not_found` sinon.

### 9.3 Vérifier la limite

Avec `MAX_ACTIVE_SESSIONS=2`, se connecter trois fois, puis :

```sql
SELECT id, created_at, revoked_at
FROM sessions
WHERE user_id = <user-id>
ORDER BY created_at DESC;
```

Attendu : au plus 2 lignes avec `revoked_at IS NULL`.

### 9.4 Vérifier le GC

```sql
UPDATE sessions
   SET created_at = now() - interval '40 days',
       expires_at = now() - interval '31 days'
 WHERE id = <session-id>;
```

Puis attendre le passage du worker ou lancer manuellement :

```bash
cd services/identity/api
DATABASE_URL='postgres://.../campus_identity?sslmode=disable' go run ./cmd/worker
```

La session doit être supprimée après le tick de nettoyage.

---

## 10. Rollback

1. Migration :

```bash
make migrate-down-identity
```

`0005` down restaure la FK `replaced_by` sans `ON DELETE SET NULL` et supprime les index.

2. Code :

- restaurer les fichiers modifiés ;
- régénérer `generated.go` et le client TS ;
- supprimer `session_integration_test.go` si nécessaire.

3. Compose/Docker :

- retirer `identity-worker` du compose et la ligne `worker` du Dockerfile.

---

## 11. Limites restantes

- Révoquer une session ne révoque pas immédiatement l’access token JWT déjà émis : il reste valide jusqu’à 10 minutes.
- `last_used_at` est mis à jour au login/refresh, pas à chaque requête HTTP.
- Le GC dépend du worker : s’il n’est pas déployé, les sessions ne sont jamais supprimées.
- Le worker outbox marque toujours les événements `sent` sans publication réelle (dette séparée).
- Les endpoints `/sessions` ne sont pas encore routés par le gateway.
- La limite de sessions s’applique aux nouveaux logins ; elle ne révoque pas rétroactivement un excédent existant tant qu’un nouveau login n’a pas lieu.

---

## 12. Checklist PR

- [ ] Réutilisation d’un refresh token → famille révoquée, `401`.
- [ ] Échec DB de `RevokeFamily` → `500`.
- [ ] `last_used_at` non nul après login et refresh.
- [ ] 3 logins avec limite 2 → 2 sessions actives.
- [ ] `GET /sessions` ne renvoie pas les sessions révoquées/expirées.
- [ ] `current: true` sur la session du token courant.
- [ ] `DELETE /sessions/{id}` d’un autre utilisateur → `404`.
- [ ] `DELETE /sessions/{id}` de sa session → `204`.
- [ ] GC supprime les sessions anciennes.
- [ ] Migration `0005` up/down testée.
- [ ] `go build ./...`, `go test ./...`, `go vet ./...` passent.
