# Technology Stack

## Core Technologies

### Backend Framework
- **Language**: Go 1.22.3
- **HTTP Router**: Chi v5 (lightweight, fast HTTP router)
- **Architecture**: Clean Architecture with dependency injection
- **Pattern**: API Gateway with microservices integration

### Authentication & Authorization
- **Primary Auth**: SuperTokens (passwordless magic link authentication)
- **Session Management**: Redis-based sessions
- **Authorization**: Casbin RBAC (Role-Based Access Control)
- **API Keys**: Custom implementation for superadmin access
- **JWT**: Custom JWT handling for session tokens

### Data Storage

#### FHIR Data
- **Server**: Blaze FHIR Server (Samply/Blaze)
- **Standard**: FHIR R4 compliant
- **Access**: Internal only, proxied through backend
- **Resources**: Patient, Practitioner, Organization, Appointment, Observation, etc.

#### Session & Cache
- **Technology**: Redis
- **Usage**: User sessions, rate limiting counters, temporary data
- **Features**: TTL support, pub/sub capabilities

### External Integrations

#### Payment Gateway
- **Provider**: OY! Indonesia
- **Services**: Payment routing, status checking
- **Features**: Multiple payment methods, transaction tracking

#### Messaging
- **Technology**: RabbitMQ
- **Queues**: Email delivery, WhatsApp notifications
- **Pattern**: Asynchronous message processing

## Key Dependencies

### Core Framework
```go
github.com/go-chi/chi/v5 v5.1.0          // HTTP router
github.com/go-chi/cors v1.2.1            // CORS middleware
github.com/supertokens/supertokens-golang // Authentication
```

### Database & Storage
```go
github.com/redis/go-redis/v9 v9.5.3      // Redis client
```

### Authorization & Security
```go
github.com/casbin/casbin/v2 v2.108.0     // RBAC enforcement
github.com/golang-jwt/jwt/v4 v4.5.0      // JWT handling
golang.org/x/crypto v0.24.0              // Cryptographic functions
```

### Configuration & Utilities
```go
github.com/spf13/viper v1.19.0           // Configuration management
github.com/joho/godotenv v1.5.1          // Environment variables
github.com/fsnotify/fsnotify v1.7.0      // File system notifications
```

### Logging & Monitoring
```go
go.uber.org/zap v1.27.0                  // Structured logging
```

### HTTP & Networking
```go
github.com/andybalholm/brotli v1.2.0     // Brotli compression
github.com/go-chi/httprate v0.9.0        // Rate limiting
golang.org/x/time v0.6.0                 // Time utilities
```

### Data Processing
```go
github.com/tidwall/gjson v1.18.0         // JSON parsing
github.com/goccy/go-json v0.10.3         // Fast JSON encoding
github.com/go-playground/validator/v10   // Input validation
```

### Messaging & Communication
```go
github.com/rabbitmq/amqp091-go v1.10.0   // RabbitMQ client
gopkg.in/gomail.v2 v2.0.0                // Email sending
```

## Development Setup

### Prerequisites
- Go 1.22.3 or later
- Docker & Docker Compose
- Redis (for sessions/cache)

### Local Development Stack
```yaml
# docker-compose.yml services
postgres-core-konsulin:    # SuperTokens database
redis-core-konsulin:       # Session storage
blaze-core-konsulin:       # FHIR server
supertokens-core-konsulin: # Authentication service
```

### Configuration Management
- **Environment Variables**: `.env` file for local development
- **YAML Config**: `config.{env}.yaml` for structured configuration
- **Viper**: Dynamic configuration loading with environment override
- **Validation**: Required configuration validation on startup

### Build System
- **Multi-stage Docker**: Separate vendor and application builds
- **Build Scripts**: `build.sh` and `build-vendor.sh` for automation
- **Deployment**: Coolify deployment triggered by GitHub action

## Performance Optimizations

### Connection Pooling
- **HTTP Clients**: Reusable clients with connection pooling
- **Redis**: Connection pool management

### Caching Strategy
- **Session Cache**: Redis for user sessions
- **Rate Limiting**: Redis-based token bucket
- **Response Caching**: Conditional caching for public resources

### Compression
- **Brotli**: Response compression for bandwidth optimization
- **Gzip**: Fallback compression support

### Rate Limiting
- **Algorithm**: Token bucket implementation
- **Storage**: Redis-based counters
- **Granularity**: Per-user and per-API-key limits

## Security Features

### Input Validation
- **Validator**: Struct-based validation with custom rules
- **Sanitization**: Input sanitization for security
- **FHIR Validation**: Resource validation against FHIR schemas

### Request Security
- **CORS**: Configurable cross-origin resource sharing
- **Body Limits**: Request body size limitations
- **Timeout**: Request timeout handling

### Data Protection
- **Encryption**: Data encryption at rest and in transit
- **Access Control**: Fine-grained RBAC permissions
- **Audit Logging**: Comprehensive request/response logging

## Deployment Architecture

### Containerization
- **Base Image**: Alpine Linux for minimal footprint
- **Multi-stage**: Optimized build process
- **Health Checks**: Container health monitoring

### Production Infrastructure
- **Domain**: api.konsulin.care
- **SSL**: Let's Encrypt certificates
- **Reverse Proxy**: Nginx with automatic SSL
- **Monitoring**: Structured logging with log rotation

### Environment Management
- **Development**: Local Docker Compose stack
- **Staging**: Containerized deployment with test data
- **Production**: Full infrastructure with monitoring and backups

## Testing Strategy

### Unit Testing
- **Framework**: Go standard testing package
- **Mocking**: Interface-based mocking for dependencies
- **Coverage**: Comprehensive test coverage for business logic

### Integration Testing
- **RBAC Testing**: Authorization policy testing
- **API Testing**: End-to-end API testing
- **Database Testing**: Repository layer testing

### Performance Testing
- **Load Testing**: API endpoint performance testing
- **Stress Testing**: System limits and bottleneck identification