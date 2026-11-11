# Identity Service Architecture & Workflows

This document contains visual architectural specifications and sequence diagrams for the Campus++ Identity Service.

---

## 1. Overall System Architecture & Request Pipeline

```mermaid
graph TD
    Client(["Client / Frontend / Gateway"])

    subgraph IdentityService ["services/identity/api"]
        subgraph Middlewares ["Middleware Pipeline"]
            Recovery["Recovery"]
            Logger["Logger"]
            ReqID["RequestIDMiddleware<br/>(RFC 4122 UUID)"]
            CORS["CORSMiddleware"]
            AuthMW["BearerAuthMiddleware<br/>(Verifies RS256 with PublicKey)"]
        end

        subgraph Handlers ["internal/handler"]
            AuthH["AuthHandler<br/>- /auth/login<br/>- /auth/refresh<br/>- /auth/logout<br/>- /me<br/>- /healthz"]
        end

        subgraph Service ["internal/service"]
            AuthSvc["AuthService<br/>- Verify bcrypt password<br/>- Validate user status<br/>- Issue RS256 JWT<br/>- Hash & Rotate Opaque Refresh Tokens"]
        end

        subgraph Repositories ["internal/repository"]
            UserRepo["UserRepository<br/>(FindByEmail, FindByID, FindMeByID)"]
            SessRepo["SessionRepository<br/>(Create, FindByTokenHash, Rotate)"]
        end
    end

    subgraph PlatformGo ["platform/go/auth"]
        JWT["GenerateAccessToken / VerifyAccessToken<br/>(RS256 Private/Public Keys)"]
        Crypto["HashPassword / CheckPassword (Bcrypt)<br/>GenerateOpaqueRefreshToken"]
    end

    subgraph Storage ["PostgreSQL (infra/compose)"]
        UsersDB[("users table")]
        SessionsDB[("sessions table<br/>(BYTEA token_hash, family_id)")]
    end

    %% Flow connections
    Client -->|HTTP Request| ReqID
    ReqID --> CORS --> Recovery --> Logger --> AuthMW
    AuthMW --> AuthH
    AuthH --> AuthSvc
    AuthSvc <--> PlatformGo
    AuthSvc --> UserRepo
    AuthSvc --> SessRepo
    UserRepo <--> UsersDB
    SessRepo <--> SessionsDB
```

---

## 2. Authentication Flow (`POST /auth/login`)

```mermaid
sequenceDiagram
    autonumber
    actor Client
    participant H as AuthHandler
    participant S as AuthService
    participant UR as UserRepository
    participant SR as SessionRepository
    participant PG as platform/go/auth
    participant DB as PostgreSQL

    Client->>H: POST /auth/login {email, password}
    H->>S: Login(ctx, email, password, ip, ua)
    
    %% User lookup
    S->>UR: FindByEmail(ctx, email)
    UR->>DB: SELECT * FROM users WHERE email = $1
    DB-->>UR: user record
    UR-->>S: *model.User

    %% Password check
    S->>PG: CheckPassword(plainPassword, password_hash)
    PG-->>S: true / false (bcrypt compare)
    alt Invalid Password or Suspended User
        S-->>H: ErrInvalidCredentials / ErrAccountSuspended
        H-->>Client: 401 Unauthorized / 403 Forbidden (RFC 9457 Problem Details)
    end

    %% Token generation
    S->>PG: GenerateOpaqueRefreshToken() (32 bytes entropy)
    PG-->>S: rawRefreshToken
    S->>PG: HashRefreshToken(rawRefreshToken)
    PG-->>S: sha256 bytea hash

    %% Create session
    S->>SR: Create(ctx, session with FamilyID & token_hash)
    SR->>DB: INSERT INTO sessions (...) RETURNING id
    DB-->>SR: session_id

    %% Sign JWT
    S->>PG: GenerateAccessToken(user_id, sid, campus_id, role, family_id, privateKey)
    PG-->>S: signed RS256 JWT string

    %% Return
    S-->>H: TokenPair {accessToken, expiresIn, refreshToken, tokenType: "Bearer"}
    H-->>Client: 200 OK + JSON TokenResponse
```

---

## 3. Server Lifecycle & Context Supervision (`cmd/server/main.go`)

```mermaid
flowchart TD
    Start([main Starts]) --> InitSignal["signal.NotifyContext(context.Background, SIGINT, SIGTERM)"]
    InitSignal --> LoadConfig["config.Load() & pgxpool.NewWithConfig(ctx)"]
    LoadConfig --> DBCheck{"dbPool.Ping(ctx)"}
    
    DBCheck -- Failure --> FatalExit["Exit(1) with Error"]
    DBCheck -- Success --> LoadKeys["Load/Generate RSA Keys (private.pem / public.pem)"]
    
    LoadKeys --> InitRouter["Setup Gin Router, Handlers & Middlewares"]
    InitRouter --> StartGoRoutine["Spawn Background Goroutine:<br/>srv.ListenAndServe()"]
    
    StartGoRoutine --> SelectBlock{"select block"}
    SelectBlock -- "serverErr (Port conflict / startup error)" --> Abort["Exit with Server Listen Error"]
    SelectBlock -- "<-ctx.Done() (SIGINT/SIGTERM received)" --> TriggerShutdown["Start Graceful Shutdown"]
    
    TriggerShutdown --> CreateTimeout["shutdownCtx = context.WithTimeout(Background, 5s)"]
    CreateTimeout --> SrvShutdown["srv.Shutdown(shutdownCtx)"]
    SrvShutdown -- Pending requests finish --> CleanStop([Process Exits 0 Cleanly])
    SrvShutdown -- Exceeds 5s --> ForceStop([DeadlineExceeded: Force Cut Connections])
```
