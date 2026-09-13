# Repository Structure

```
konsulin-service/
├── AGENTS.md                  # Agent entry point — start here
├── README.md                  # Project README (installation, features)
│
├── cmd/                       # Entry points
│   ├── http/main.go           # HTTP API server (main binary)
│   ├── migration/main.go      # DB migration tool
│   └── example/main.go        # Reference code examples
│
├── internal/
│   ├── app/                   # Application layer (Clean Architecture)
│   │   ├── config/            # Env + YAML config structs
│   │   ├── contracts/         # Interface definitions (24 interfaces)
│   │   ├── services/          # Business logic
│   │   │   ├── core/          # Auth, users, payments, slots, webhooks
│   │   │   ├── fhir_spark/    # FHIR resource clients (14 resource types)
│   │   │   └── shared/        # Shared services (JWT, mailer, redis, etc.)
│   │   ├── delivery/http/     # HTTP layer
│   │   │   ├── controllers/   # 9 HTTP controllers
│   │   │   ├── middlewares/   # 9 middleware components
│   │   │   ├── routers/       # Route setup & registration
│   │   │   └── handlers/      # HTTP handler utilities
│   │   ├── drivers/           # Infrastructure wrappers
│   │   │   ├── database/      # DB connection drivers
│   │   │   ├── logger/        # Zap logger wrapper
│   │   │   ├── messaging/     # RabbitMQ wrapper
│   │   │   └── thirdparty/    # External service integration
│   │   └── models/            # Domain entities (BSON-tagged)
│   │
│   └── pkg/                   # Shared packages
│       ├── fhir_http_client/  # Standard FHIR HTTP client
│       ├── fhir_dto/          # FHIR R4 resource DTOs
│       ├── dto/               # Application request/response DTOs
│       ├── constvars/         # Constants & context keys
│       ├── exceptions/        # Typed errors with HTTP mapping
│       ├── queries/           # SQL query constants
│       └── utils/             # General utility functions
│
├── resources/                 # Static configuration
│   ├── rbac_model.conf        # Casbin RBAC model
│   ├── rbac_policy.csv        # RBAC policy rules
│   └── auth/                  # Auth-related test scripts
│
├── api/                       # API specification files
├── build/                     # Docker build files
│   └── Dockerfile             # Multi-stage Docker build
│
├── scripts/                   # Utility scripts
├── _data/                     # Data directory (placeholder)
│
├── .github/workflows/         # GitHub Actions CI/CD
│
├── docs/                      # Agent documentation
│   ├── AGENTS.md              # Documentation index
│   ├── ARCHITECTURE.md        # Architecture + tech stack
│   ├── STANDARDS.md           # Coding standards
│   ├── KNOWN-PITFALLS.md      # Common pitfalls
│   └── STRUCTURE.md           # This file
│
├── docker-compose.yml         # Local dev services
├── .env.example               # Environment variable template
├── build.sh                   # Build script
├── build-vendor.sh            # Vendor dependency builder
├── go.mod / go.sum            # Go module definitions
└── Makefile                   # (optional, project may use scripts/)
```
