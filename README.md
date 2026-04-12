# go-utility

A collection of production-ready Go utility packages for building backend services — config loading, database wrappers, message brokers, HTTP frameworks, authentication, logging, and more.

## Versions

| | v1 | v2 |
|---|---|---|
| Module path | `github.com/AndreeJait/go-utility` | `github.com/AndreeJait/go-utility/v2` |
| Go version | 1.22 | 1.25 |
| Import | `import "github.com/AndreeJait/go-utility/..."` | `import "github.com/AndreeJait/go-utility/v2/..."` |
| Status | Legacy | Active development |

> **New projects should use v2.** v1 is preserved for backwards compatibility.

## Installation

```bash
# v2 (recommended)
go get github.com/AndreeJait/go-utility/v2@latest

# v1 (legacy)
go get github.com/AndreeJait/go-utility@latest
```

---

## v2 Packages

### Authentication

**`authw`** — Framework-agnostic authentication and authorization with JWT, Basic Auth, and RBAC support.

```go
import "github.com/AndreeJait/go-utility/v2/authw"
```

```go
// JWT authentication
jwtAuth := authw.NewJWT(&authw.JWTConfig{
    JWT:       jwtw.New(&jwtw.Config{SecretKey: "your-secret"}),
    NewClaims: func() jwtv5.Claims { return &jwtw.MyClaims[MyUserData]{} },
    ExtractResult: func(claims jwtv5.Claims) *authw.Result {
        c := claims.(*jwtw.MyClaims[MyUserData])
        return &authw.Result{UserID: c.Data.ID, Username: c.Data.Username, Data: c.Data}
    },
})

// Basic authentication (static users)
basicAuth := authw.NewBasicAuth(&authw.BasicAuthConfig{
    StaticUsers: map[string]string{"admin": "password"},
})

// Basic authentication (custom validator)
basicAuth := authw.NewBasicAuth(&authw.BasicAuthConfig{
    Validator: func(username, password string) (*authw.Result, error) {
        // lookup in database, etc.
    },
})

// Access auth result in handlers
result := authw.FromContext(ctx)
result.GetUserID()  // string
result.Username     // string
result.Roles        // []string
result.Permissions  // []string
result.Data         // your custom payload
```

**RBAC — Role & Permission Authorization with Cache**

```go
// Setup RBAC with a role fetcher and local cache
rbac := authw.NewRBAC(&authw.RBACConfig{
    RoleFetcher: func(ctx context.Context, userID string) ([]string, error) {
        return db.GetUserRoles(ctx, userID) // fetch from your database
    },
    Cache:    authw.NewLocalCache(),   // or authw.NewRedisCache(redisClient)
    CacheTTL: 600,                      // 10 minutes
})

// Register role→permission mappings (used when no PermissionFetcher is set)
rbac.RegisterRole("admin", "users:read", "users:write", "users:delete")
rbac.RegisterRole("editor", "users:read", "users:write")

// Optional: custom permission fetcher (overrides role→permission derivation)
rbac = authw.NewRBAC(&authw.RBACConfig{
    PermissionFetcher: func(ctx context.Context, userID string) ([]string, error) {
        return db.GetUserPermissions(ctx, userID)
    },
    Cache:    authw.NewLocalCache(),
    CacheTTL: 300,
})

// Check authorization in your code
ok, _ := rbac.CheckRole(ctx, userID, "admin")           // role check
ok, _ := rbac.CheckPermission(ctx, userID, "users:write") // permission check

// Invalidate cache when user roles change
rbac.InvalidateUser(ctx, userID)
```

**`jwtw`** — JWT token creation and parsing (HS256, upgradeable).

```go
import "github.com/AndreeJait/go-utility/v2/jwtw"
```

```go
mgr := jwtw.New(&jwtw.Config{SecretKey: "your-secret"})

// Create token
claims := jwtw.NewClaims(userData{ID: "1", Name: "alice"}, 5*time.Minute, "myapp", "auth")
token, _ := mgr.Create(claims)

// Parse token
var parsed jwtw.MyClaims[userData]
mgr.Parse(tokenStr, &parsed)
// parsed.Data.ID == "1"
```

### HTTP Frameworks

All three wrappers share the same patterns: `API` interface, `Bind()` adapter, `ApiWrap()` executor, `ParsePagination()`, `AuthMiddleware()`, `RequireRole()`, and `RequirePermission()`.

**`httpw/echow`** — Echo v5 wrapper

```go
import "github.com/AndreeJait/go-utility/v2/httpw/echow"
```

```go
e := echow.New(&echow.Config{
    DebugMode:    true,
    Authenticator: jwtAuth,  // optional global auth
})

// Public route
e.GET("/profile", echow.Bind(func(c *echo.Context) (any, error) {
    auth := authw.FromContext(c.Request().Context())
    return auth.GetUserID(), nil
}))

// Role-protected route
e.GET("/admin", handler, echow.RequireRole(rbac, "admin"))

// Permission-protected route
e.POST("/users", handler, echow.RequirePermission(rbac, "users:write"))
```

**`httpw/ginw`** — Gin wrapper

```go
import "github.com/AndreeJait/go-utility/v2/httpw/ginw"
```

```go
r := ginw.New(&ginw.Config{DebugMode: true, Authenticator: jwtAuth})

r.GET("/profile", ginw.Bind(func(c *gin.Context) (any, error) {
    return authw.FromContext(c.Request.Context()).GetUserID(), nil
}))

// Role-protected route
r.GET("/admin", ginw.RequireRole(rbac, "admin"), handler)

// Permission-protected route
r.POST("/users", ginw.RequirePermission(rbac, "users:write"), handler)
```

**`httpw/muxw`** — Gorilla Mux wrapper

```go
import "github.com/AndreeJait/go-utility/v2/httpw/muxw"
```

```go
r := muxw.New(&muxw.Config{DebugMode: true, Authenticator: jwtAuth})

r.HandleFunc("/profile", muxw.Bind(func(r *http.Request) (any, error) {
    return authw.FromContext(r.Context()).GetUserID(), nil
}))

// Permission-protected sub-router
api := r.PathPrefix("/api").Subrouter()
api.Use(muxw.RequirePermission(rbac, "api:access"))
```

### Response & Errors

**`statusw`** — Protocol-agnostic error types with logical codes.

```go
import "github.com/AndreeJait/go-utility/v2/statusw"
```

```go
// Predefined sentinels (clone before use)
statusw.InvalidCredential.WithCustomMessage("Invalid token")    // 401000 → HTTP 401
statusw.InvalidAccess.WithCustomMessage("Not allowed")         // 403000 → HTTP 403
statusw.NotFound.WithCustomMessage("User not found")           // 404000 → HTTP 404
statusw.InvalidReqParam.WithError(err)                        // 400000 → HTTP 400
statusw.InternalServerError.WithError(err)                     // 500000 → HTTP 500
statusw.Conflict.WithCustomMessage("Email already exists")     // 409000 → HTTP 409

// Safe chaining (each call clones)
err := statusw.InvalidCredential.WithError(err).WithCustomMessage("Token expired")
```

**`responsew`** — Standardized API response builder.

```go
import "github.com/AndreeJait/go-utility/v2/responsew"
```

```go
responsew.Success(data, "OK")                          // BaseResponse with data
responsew.SuccessPaginated(items, total, 1, 10, "OK") // with pagination metadata
httpCode, payload := responsew.Error(err)             // auto-maps statusw → HTTP code
```

### Configuration

**`configw`** — Viper-based config loading (YAML + .env + OS env overrides).

```go
import "github.com/AndreeJait/go-utility/v2/configw"
```

```go
// Load YAML into struct (env vars override: DATABASE_HOST overrides Database.Host)
var cfg AppConfig
configw.Load("config.yaml", &cfg)

// Load .env file
configw.LoadEnv(".env")
```

### Logging & Tracing

**`logw`** — Global slog-based logger with context-aware logging via `x-log-id`.

```go
import "github.com/AndreeJait/go-utility/v2/logw"
```

```go
logw.Init(&logw.LogConfig{Level: "info", Format: logw.JSONFormat})

// Context-aware (includes x-log-id)
logw.CtxInfof(ctx, "user %s logged in", userID)

// Flat (no context)
logw.Infof("server started on :%d", port)
```

**`spanw`** — Lightweight function-level tracing.

```go
import "github.com/AndreeJait/go-utility/v2/spanw"
```

```go
spanw.Init(&spanw.SpanConfig{Enabled: true})

func HandleRequest(ctx context.Context) {
    ctx, finish := spanw.Start(ctx, "HandleRequest")
    defer finish()
    // span chain and timing tracked automatically
}
```

### Databases

**`sql/gormw`** — GORM wrapper (MySQL, PostgreSQL, SQLite).

```go
import "github.com/AndreeJait/go-utility/v2/sql/gormw"
```

```go
db, _ := gormw.Connect(ctx, &gormw.Config{Driver: gormw.DriverPostgres, DSN: dsn})
gormw.Transaction(ctx, db, func(tx *gorm.DB) error { ... })
```

**`sql/sqlxw`** — SQLX wrapper (MySQL, PostgreSQL, SQLite) with debug query logging.

```go
import "github.com/AndreeJait/go-utility/v2/sql/sqlxw"
```

```go
db, _ := sqlxw.Connect(ctx, &sqlxw.Config{Driver: sqlxw.DriverMySQL, DSN: dsn})
sqlxw.Transaction(ctx, db, func(tx sqlxw.ExtContext) error { ... })
```

**`sql/migratew`** — Database migration wrapper (golang-migrate).

```go
import "github.com/AndreeJait/go-utility/v2/sql/migratew"
```

```go
//go:embed migrations
var migrationFS embed.FS

m, _ := migratew.New(db, migratew.Postgres, migrationFS, "migrations",
    migratew.WithSchema("tenant_auth"))
m.Up()
```

**`no-sql/mongow`** — MongoDB v2 with command monitoring.

```go
import "github.com/AndreeJait/go-utility/v2/no-sql/mongow"

client, _ := mongow.Connect(ctx, &mongow.Config{URI: "mongodb://localhost:27017", DebugMode: true})
```

**`no-sql/redisw`** — Redis v9 with debug hooks.

```go
import "github.com/AndreeJait/go-utility/v2/no-sql/redisw"

rdb, _ := redisw.Connect(ctx, &redisw.Config{Address: "localhost:6379"})
```

**`no-sql/elasticw`** — Elasticsearch v8 with debug transport.

```go
import "github.com/AndreeJait/go-utility/v2/no-sql/elasticw"

es, _ := elasticw.Connect(ctx, &elasticw.Config{Addresses: []string{"http://localhost:9200"}})
```

### Message Brokers

**`brokerw`** — Producer/Consumer abstraction with Kafka, NSQ, RabbitMQ, and RocketMQ implementations.

```go
import "github.com/AndreeJait/go-utility/v2/brokerw"
import "github.com/AndreeJait/go-utility/v2/brokerw/kafkaw"
```

```go
producer := kafkaw.NewProducer([]string{"localhost:9092"})
producer.Send(ctx, "topic", []byte("key"), []byte("payload"))

consumer, _ := kafkaw.NewConsumer([]string{"localhost:9092"}, "group")
consumer.Consume(ctx, "topic", func(ctx context.Context, msg *brokerw.Message) error {
    // handle message
})
```

Same pattern for `nsqw`, `rabbitmqw`, `rocketmqw` sub-packages.

### Cloud Storage

**`storagew`** — Storage abstraction with MinIO, AWS S3, GCS, and Huawei OBS.

```go
import "github.com/AndreeJait/go-utility/v2/storagew"
import "github.com/AndreeJait/go-utility/v2/storagew/miniow"
```

```go
storage, _ := miniow.New(&miniow.Config{Endpoint: "localhost:9000", AccessKeyID: "minio", SecretAccessKey: "minio123"})
storage.Upload(ctx, "bucket", "file.txt", reader, size, "text/plain")
rc, _ := storage.Download(ctx, "bucket", "file.txt")
```

Same pattern for `awsw`, `gcsw`, `huaweiw` sub-packages.

### Bots

**`botw`** — Bot abstraction with Discord and Telegram.

```go
import "github.com/AndreeJait/go-utility/v2/botw"
import "github.com/AndreeJait/go-utility/v2/botw/discordw"
```

```go
bot, _ := discordw.New("bot-token")
bot.SendMessage(ctx, "channel-id", "Hello!")
bot.Listen(ctx, func(ctx context.Context, msg *botw.Message) error {
    // handle incoming message
})
```

### Scheduling & Concurrency

**`cronw`** — Cron scheduler with middleware chaining.

```go
import "github.com/AndreeJait/go-utility/v2/cronw"
```

```go
scheduler := cronw.New()
scheduler.Register("0 9 * * *", handler, middleware1, middleware2)
scheduler.Start()
```

**`goroutinew`** — Concurrent task orchestrator with bounded worker pools.

```go
import "github.com/AndreeJait/go-utility/v2/goroutinew"
```

```go
o := goroutinew.New(goroutinew.WithMaxWorkers(5), goroutinew.WithTimeout(10*time.Second))
o.AddStep("fetch", fetchFunc)
o.AddInput("fetch", userID)
o.Run(ctx)
```

### Email

**`emailw`** — SMTP emailer with templates and attachments.

```go
import "github.com/AndreeJait/go-utility/v2/emailw"
```

```go
mailer := emailw.New("smtp.host", 587, "user", "pass", "from@example.com")
body, _ := emailw.ParseTemplateFile("template.html", data)
mailer.Send(ctx, emailw.Message{To: []string{"to@example.com"}, Subject: "Hi", Body: body, IsHTML: true})
```

### State Machine

**`statemachinew`** — Fluent FSM builder with guards and hooks.

```go
import "github.com/AndreeJait/go-utility/v2/statemachinew"
```

```go
sm := statemachinew.New()
sm.On("submit").From("draft").To("review").Guard(hasRole("reviewer")).Post(notifyReviewer)
sm.On("approve").From("review").To("approved")
newState, _ := sm.Fire(ctx, "submit", "draft", entity)
```

### Infrastructure

**`gracefulw`** — Graceful shutdown with OS signal handling.

```go
import "github.com/AndreeJait/go-utility/v2/gracefulw"
```

```go
gracefulw.Register("db", db.Disconnect)
gracefulw.Register("redis", redis.Disconnect)
gracefulw.Start(func() error { return http.ListenAndServe(":8080", r) }, 10*time.Second)
```

**`localcachew`** — In-memory cache with TTL and background cleanup.

```go
import "github.com/AndreeJait/go-utility/v2/localcachew"
```

```go
localcachew.Init(5 * time.Minute)
localcachew.SetKV(ctx, "key", value, localcachew.WithTTL(10*time.Minute))
val, _ := localcachew.Get(ctx, "key")
```

---

## v1 Packages (Legacy)

| Package | Description |
|---|---|
| `configw` | Generic YAML config loader keyed by mode (dev/staging/prod) |
| `copy` | Deep struct-to-struct conversion with reflection |
| `cronw` | Cron scheduler wrapper (robfig/cron v3) |
| `emailw` | SMTP email via gomail.v2 |
| `errow` | Error types with numeric codes |
| `goroutine` | Concurrent pipeline with batch processing |
| `gracefull` | Graceful shutdown with OS signals |
| `jwt` | JWT token create/parse (HS256, golang-jwt v4) |
| `loggerw` | Logrus logger + Echo v4 middleware |
| `nosql/mongodb` | MongoDB v1 CRUD and aggregation |
| `nsqw` | NSQ producer/consumer wrapper |
| `password` | Bcrypt password hashing |
| `redisw` | Redis connection helper (go-redis v8) |
| `refactor` | Bulk string replacement across files |
| `response` | Echo v4 JSON response helpers |
| `sqlw/postgres` | PostgreSQL wrapper (pgx v5 + custom converter) |
| `stringw` | Case-insensitive string-in-slice check |
| `timew` | Timezone-aware time utilities |
| `tracer` | Request tracing with spans and timeline |
| `utils/converter` | Reflection-based row-to-struct converter |

---

## Architecture

### Interface + Strategy Pattern

Every major v2 capability is defined by an interface at the package root, with concrete implementations in sub-packages:

```
brokerw.Producer / Consumer  →  kafkaw, nsqw, rabbitmqw, rocketmqw
storagew.Storage             →  miniow, awsw, gcsw, huaweiw
botw.Bot                     →  discordw, telegramw
authw.Authenticator          →  jwt.go, basic.go
authw.Cache                 →  localCache (localcachew), redisCache (go-redis)
```

Single-implementation packages use the `interface + unexported struct` pattern:

```
jwtw.JWT       →  jwtManager
emailw.Emailer →  smtpEmailer
cronw.Scheduler →  cronScheduler
```

### Error Pipeline

```
statusw.Error (protocol-agnostic, logical codes)
  → responsew.Error() (maps to HTTP status code, builds BaseResponse JSON)
  → HTTP framework middleware catches and formats the response
```

Auth failures return `statusw.InvalidCredential` (401000) which automatically maps to HTTP 401. Authorization failures return `statusw.InvalidAccess` (403000) which maps to HTTP 403.

### Auth + RBAC Pipeline

```
Request
  → AuthMiddleware (authenticates, sets authw.Result in context)
  → RequireRole / RequirePermission (authorizes via RBAC)
    → Check context Result.Roles / Permissions first
    → Check cache (localcachew or Redis)
    → Call user-provided fetcher (DB/API)
    → Cache result for subsequent requests
    → Return 403 if unauthorized
  → Handler (accesses authw.FromContext(ctx) for user identity)
```

### Context Propagation

The `x-log-id` is injected at the HTTP inbound layer and flows through all downstream calls. Auth results are injected via `authw.WithResult()` and retrieved with `authw.FromContext()`. RBAC middleware reads user identity from the context Result.

---

## Development

```bash
# Run v2 tests
cd v2 && go test ./...

# Run a single package
cd v2 && go test ./jwtw/

# Run a specific test
cd v2 && go test -run TestJWT_CreateAndParse_Success ./jwtw/

# Run v1 tests
go test ./...
```

Dependencies are vendored. After adding a dependency:

```bash
cd v2 && go mod tidy && go mod vendor
```

## Release

Pushing to `master` automatically triggers the CI workflow (`.github/workflows/tag-on-push-master.yaml`) which:
1. Calculates the next semver tag (patch bump; patch >= 9 rolls to minor; minor >= 10 rolls to major)
2. Creates and pushes the tag
3. Generates a changelog from commit messages
4. Creates a GitHub Release

## License

This project is licensed under the terms found in the [LICENSE](LICENSE) file.