# Housing — Cleaning request avec créneau matin/après-midi

**Date :** 2026-11-28  
**Périmètre :** `services/housing` (backend uniquement)  
**Statut :** migration + contrat + queries + service + handler `createCleaningRequest` implémentés et testés unitairement.  
**Non inclus :** JWT/middleware, `cmd/server`, tests d'intégration DB, scheduler shifts/tasks, algorithme de capacité.

---

## 1. Objectif

Permettre à un étudiant de créer une demande de ménage en choisissant un créneau :

- `morning`
- `afternoon`

La demande reste liée à :

- son **stay actif** (unité dérivée, jamais fournie par le client) ;
- la date d'exécution **strictement demain** ;
- la règle **1 demande par unité et par semaine ISO**.

Le créneau est un **bucket de demi-journée**, pas un timestamp. Les heures réelles d'exécution restent portées par `cleaning_shifts.start_time` / `end_time`.

---

## 2. Décisions prises

| Sujet | Décision |
|---|---|
| Type du créneau | Enum PostgreSQL `cleaning_slot` (`morning`, `afternoon`), pas un `TIMESTAMPTZ` |
| Colonne | `cleaning_requests.slot`, `NOT NULL`, **sans default** |
| Données existantes | Backfill unique en `morning` |
| Date | `scheduledDate` optionnel ; si omis → demain ; sinon doit être exactement demain |
| Fuseau | Le service doit être configuré sur `Africa/Casablanca` ; le calcul “demain” se fait en date civile locale |
| Semaine du quota | Calculée à partir de `scheduled_date` (date d'exécution), pas de la date de création |
| Quota | Inchangé : `UNIQUE(dorm_unit_id, year, iso_week) WHERE status <> 'cancelled'` |
| Pas de stay actif | `422` (le contrat ne déclare pas de `404` pour `POST /cleaning-requests`) |
| Quota déjà pris | `409` |
| Rôle autorisé | `role = student` uniquement ; les autres rôles → `403` |
| Erreurs 422 | `ValidationProblem` avec `errors[]` obligatoire |
| Nom du DTO d'entrée | `CleaningRequestInput` (renommé depuis `CreateCleaningRequestRequest`) |
| Body | Requis : le client doit envoyer au minimum `{"slot":"morning"}` |

---

## 3. Base de données

### Migration `0005_cleaning_request_slot`

Fichiers :

- `services/housing/api/migrations/0005_cleaning_request_slot.up.sql`
- `services/housing/api/migrations/0005_cleaning_request_slot.down.sql`

Up :

1. crée l'enum `cleaning_slot` (`morning`, `afternoon`) ;
2. ajoute `cleaning_requests.slot cleaning_slot` ;
3. backfill les lignes existantes en `morning` ;
4. passe la colonne en `NOT NULL` ;
5. ne met **aucun default** : une insertion sans slot échoue volontairement.

Down :

1. supprime la colonne ;
2. supprime le type enum.

> Le quota hebdomadaire reste sur `(dorm_unit_id, year, iso_week)`.
> Le slot ne crée **pas** une deuxième demande autorisée dans la même semaine.

---

## 4. Contrat OpenAPI

Fichier : `services/housing/contracts/openapi.yaml`

Changements :

- nouveau schéma `CleaningSlot` :
  ```yaml
  CleaningSlot:
    type: string
    enum: [morning, afternoon]
  ```
- `CreateCleaningRequestRequest` renommé en `CleaningRequestInput` ;
- `slot` ajouté et **requis** dans `CleaningRequestInput` ;
- `scheduledDate` reste optionnel ;
- `slot` ajouté et **requis** dans `CleaningRequest` (réponse) ;
- `requestBody.required` passe à `true` pour `POST /housing/cleaning-requests`.

Code régénéré :

- `services/housing/api/internal/handler/generated.go`
- Types générés :
  - `handler.CleaningRequestInput`
  - `handler.CleaningSlot`
  - `handler.CleaningRequest.Slot`
  - `CreateCleaningRequestJSONRequestBody = CleaningRequestInput`

Constantes générées actuelles : `handler.Morning`, `handler.Afternoon`.
Option cosmétique non faite : `x-enum-varnames` pour obtenir `CleaningSlotMorning` / `CleaningSlotAfternoon`.

---

## 5. Modèle

Fichiers :

- `internal/model/cleaning.go`
- `internal/model/errors.go`

Ajouts :

- type `CleaningSlot` + `Valid()` ;
- champ `Slot CleaningSlot` dans `CleaningRequest` ;
- erreurs métier :
  - `ErrNoActiveStay`
  - `ErrNotTomorrow`
  - `ErrInvalidSlot`
  - `ErrWeeklyQuotaExceeded`
  - `ErrTaskNotFound`
  - `ErrInvalidTaskTransition`
  - `ErrAssignmentExists`
  - `ErrResidenceNotFound`
  - `ErrInvalidInput`

Types domaine : `CleaningRequest`, `CleaningShift`, `CleaningTask`, `HousekeeperAssignment`, `TaskFilter`.

---

## 6. Repository

Fichier : `internal/repository/cleaning.go`  
Tests de mapping : `internal/repository/cleaning_error_test.go`

### Routes couvertes

| Route | Méthode repository | Particularité |
|---|---|---|
| `POST /cleaning-requests` | `CreateFromActiveStay` | `INSERT ... SELECT` depuis `stays` actif ; slot inclus ; quota mappé en 409 |
| `GET /me/cleaning-requests` | `ListRequestsByStudent` | `WHERE student_id = $1` |
| `GET /me/shifts` | `ListShiftsByOperator` | `operator_id` + `shift_date` |
| `GET /cleaning-tasks` (opérateur) | `ListTasksForOperator` | scope `s.operator_id = $1` + filtres |
| `GET /cleaning-tasks` (superviseur) | `ListTasksForSupervisor` | scope `s.campus_id = $1` + filtres |
| `POST /cleaning-tasks/{id}/start` | `StartTask` | `pending → in_progress` |
| `POST /cleaning-tasks/{id}/complete` | `CompleteTask` | `in_progress → done` + `completed_at = now()` |
| `POST /housekeeper-assignments` | `AssignHousekeeper` | `UNIQUE(user_id)` → 409 ; FK résidence → 422 |

### Création

```sql
INSERT INTO cleaning_requests (
    dorm_unit_id, student_id, stay_id, scheduled_date, slot, status, year, iso_week
)
SELECT s.dorm_unit_id, s.student_id, s.id,
       $2::date, $3::cleaning_slot, 'pending', $4, $5
FROM stays s
WHERE s.student_id = $1
  AND s.status = 'active'
LIMIT 1
RETURNING id, dorm_unit_id, student_id, stay_id, scheduled_date, slot,
          status, year, iso_week, created_at, updated_at;
```

- 0 ligne → `ErrNoActiveStay` → 422.
- `23505` sur `uq_unit_weekly_cleaning_quota` → `ErrWeeklyQuotaExceeded` → 409.
- Aucune ligne écrite si le stay n'existe pas.

### Tâches

- `StartTask` / `CompleteTask` utilisent une transaction :
  1. `SELECT ... FOR UPDATE OF t` avec `s.operator_id = $2` ;
  2. pas de ligne → `ErrTaskNotFound` (404) ;
  3. mauvais statut → `ErrInvalidTaskTransition` (409) ;
  4. `UPDATE ... RETURNING`.
- Une tâche d'un autre opérateur est indistinguable d'une tâche inexistante → 404.
- `complete` ne stamp `completed_at` que sur `in_progress → done`.

### Mapping PostgreSQL

| Erreur | Mapping |
|---|---|
| `23505` + `uq_unit_weekly_cleaning_quota` | `ErrWeeklyQuotaExceeded` |
| `23505` + `uq_housekeeper_assignments` | `ErrAssignmentExists` |
| `23503` (FK résidence) | `ErrResidenceNotFound` |
| `23514` / classe `22` | `ErrInvalidInput` |

> `assignHousekeeper` ne peut pas vérifier le `worker_role` de la cible avec le JWT du manager.
> Le roster est traité comme **input de confiance** ; un futur endpoint interne Identity devra le valider.

---

## 7. Service

Fichier : `internal/service/cleaning.go`  
Tests : `internal/service/cleaning_test.go`

`CreateCleaningRequest(ctx, studentID, scheduledDate *time.Time, slot)` :

1. valide le slot → `ErrInvalidSlot` ;
2. résout la date :
   - `nil` → demain dans `loc` ;
   - fournie → doit être exactement demain, sinon `ErrNotTomorrow` ;
3. calcule `ISOWeek()` sur la date d'exécution ;
4. appelle `CreateFromActiveStay`.

Points importants :

- l'horloge est injectable (`now`) pour les tests ;
- `loc` est injecté (à brancher sur `Africa/Casablanca` dans `main.go`) ;
- la date retournée est une date civile à minuit UTC, pour éviter les décalages de fuseau/session DB.

Tests couverts :

- défaut demain ;
- date fournie demain ;
- aujourd'hui / hier / semaine prochaine / date lointaine → 422 ;
- slot invalide ;
- propagation `ErrNoActiveStay` ;
- `2027-01-01` → `2026-W53` ;
- frontière de fuseau : 23:30 UTC = 00:30 Casa → demain = J+2.

---

## 8. Handler

Fichiers :

- `internal/handler/auth_context.go`
- `internal/handler/problem.go`
- `internal/handler/handler.go`
- `internal/handler/cleaning.go`
- `internal/handler/cleaning_test.go`

### Claims

`ClaimsFromContext` lit :

- `userID` (`int64`)
- `role` (`string`)
- `workerRole` (`string`)
- `campusID` (`int64`)
- `sessionID` (`int64`)

Le middleware de ton binôme doit poser exactement ces clés.

### Problem Details

- `RespondProblem` : `application/problem+json`, `type`, `title`, `status`, `code`, `detail`, `requestId`.
- `RespondValidationProblem` : même chose + `errors[]` (obligatoire pour les 422).
- `requestId` est lu depuis `c.GetString("requestId")` ; le header `X-Request-Id` doit être posé par le middleware.

### `CreateCleaningRequest`

| Cas | HTTP | Code |
|---|---|---|
| Succès | 201 | `CleaningRequest` |
| Pas de token | 401 | `unauthorized` + `WWW-Authenticate` |
| Rôle non student | 403 | `forbidden` |
| JSON invalide / body absent | 400 | `invalid_body` |
| Slot invalide | 422 | `validation_failed` + `errors[slot]` |
| Date ≠ demain | 422 | `validation_failed` + `errors[scheduledDate]` |
| Pas de stay actif | 422 | `no_active_stay` + `errors[stay]` |
| Quota déjà pris | 409 | `weekly_quota_exceeded` |
| Erreur interne | 500 | `internal_error` |

Autres endpoints : stubs `501 not_implemented`.
`Healthz` est implémenté (`200 {"status":"ok"}`) pour le healthcheck Docker.

Vérification compile-time :

```go
var _ ServerInterface = (*Handler)(nil)
```

Tests handler couverts :

- 201 + champs + `X-Request-Id` ;
- date omise → nil passé au service ;
- 401 / 403 / 400 / 422 slot / 422 date / 422 no stay / 409 quota / 500 ;
- `Content-Type: application/problem+json` ;
- présence de `errors[]` et de `requestId`.

---

## 9. Tests et vérifications

Exécuté :

```bash
cd services/housing/api
go build ./...
go test ./internal/...
```

Résultat :

- build OK ;
- `internal/handler` OK ;
- `internal/service` OK ;
- `internal/repository` OK (tests de mapping d'erreurs, sans DB).

---

## 10. Fichiers ajoutés / modifiés

### Ajoutés

- `services/housing/api/migrations/0005_cleaning_request_slot.up.sql`
- `services/housing/api/migrations/0005_cleaning_request_slot.down.sql`
- `services/housing/api/internal/model/cleaning.go`
- `services/housing/api/internal/model/errors.go`
- `services/housing/api/internal/repository/cleaning.go`
- `services/housing/api/internal/repository/cleaning_error_test.go`
- `services/housing/api/internal/service/cleaning.go`
- `services/housing/api/internal/service/cleaning_test.go`
- `services/housing/api/internal/handler/auth_context.go`
- `services/housing/api/internal/handler/problem.go`
- `services/housing/api/internal/handler/handler.go`
- `services/housing/api/internal/handler/cleaning.go`
- `services/housing/api/internal/handler/cleaning_test.go`

### Modifiés

- `services/housing/contracts/openapi.yaml`
- `services/housing/api/internal/handler/generated.go` (régénéré)

### Non touchés

- `cmd/server/main.go`
- `internal/config/config.go`
- `internal/handler/middleware.go` (JWT / RequestID)
- `infra/compose/docker-compose.yml`
- `Dockerfile`
- `platform/go/*` (hors sujet)

---

## 11. Reste à faire

1. **Tests d'intégration repository** avec `TEST_DATABASE_URL` :
   - quota 409, `cancelled` libère le slot ;
   - pas de stay → aucune ligne ;
   - frontière ISO `2027-01-01` ;
   - cross-operator `shiftId` → `[]` ;
   - start/complete 404 / 409 / 200 ;
   - assignment duplicate 409 / résidence inconnue 422.
2. **Wiring serveur** (binôme) :
   - middleware JWT qui pose les clés `userID`, `role`, `workerRole`, `campusID`, `sessionID` ;
   - `RequestIDMiddleware` qui pose `requestId` + `X-Request-Id` ;
   - `NewCleaningService(repo, loc)` avec `loc = Africa/Casablanca` ;
   - `NewHandler(cleaningService)`.
3. **Appliquer la migration** :
   ```bash
   make migrate-up-housing
   ```
4. **Optionnel** : `x-enum-varnames` pour `CleaningSlotMorning` / `CleaningSlotAfternoon`.
5. **Non implémenté volontairement** :
   - algorithme de capacité matin/après-midi ;
   - génération des shifts/tâches ;
   - transitions `pending → scheduled → completed` des demandes ;
   - pagination des listes.

---

## 12. Points de vigilance

- **Pas de `morning` silencieux** : `slot` est requis dans le contrat et `NOT NULL` sans default en DB.
- **422 = `ValidationProblem`** : toujours renvoyer `errors[]`, pas un `Problem` simple.
- **`X-Request-Id`** : doit être posé par le middleware sur **toutes** les réponses, y compris 401/403/422/409/500.
- **Fuseau** : “demain” doit être calculé en `Africa/Casablanca`, pas en UTC.
- **Semaine ISO** : calculée sur `scheduled_date`, pas sur `created_at`.
- **Quota** : par unité et par semaine, pas par étudiant.
- **Index manquant** : `cleaning_shifts(campus_id)` pour le scope superviseur ; à ajouter si le volume l'exige.
- **`assignHousekeeper`** : le `worker_role` de la cible n'est pas vérifié (roster de confiance).
- **Stubs 501** : les autres endpoints existent dans `ServerInterface` mais ne sont pas fonctionnels.
