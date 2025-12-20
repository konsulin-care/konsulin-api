# System Architecture

## Overview

The Konsulin backend follows a **Clean Architecture** pattern with **API Gateway** design, serving as a centralized entry point that routes requests to appropriate internal and external services. The system is built in Go and follows microservices principles with containerized deployment.

## High-Level Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Frontend      │    │   Mobile Apps    │    │  External APIs  │
│   (React/Next)  │    │   (iOS/Android)  │    │  (Integrations) │
└─────────┬───────┘    └─────────┬────────┘    └─────────┬───────┘
          │                      │                       │
          └──────────────────────┼───────────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │    API Gateway          │
                    │    (Konsulin Backend)   │
                    │                         │
                    │  ┌─ Authentication ─┐   │
                    │  ┌─ Authorization ──┐   │
                    │  ┌─ Rate Limiting ──┐   │
                    │  ┌─ Request Routing ┐   │
                    └────────────┬────────────┘
                                 │
        ┌────────────────────────┼────────────────────────┐
        │                        │                        │
┌───────▼────────┐    ┌─────────▼──────────┐    ┌────────▼────────┐
│ FHIR Service   │    │  Payment Gateway   │    │ Webhook Service │
│ (Blaze Server) │    │  (OY! Indonesia)   │    │ (Internal/Ext)  │
│ - Internal     │    │  - External Proxy  │    │ - Mixed Access  │
│ - Not Exposed  │    │  - /pay routes     │    │ - /hook routes  │
└────────────────┘    └────────────────────┘    └─────────────────┘
```

## Core Components

### 1. API Gateway (Backend Core)
**Location**: [`cmd/http/main.go`](cmd/http/main.go)
- **Purpose**: Central request router and access controller
- **Responsibilities**:
  - Authentication via SuperTokens
  - Role-based access control (RBAC)
  - Request routing to services
  - Rate limiting and security
  - API key management for superadmin access

### 2. FHIR Service Integration
**Location**: [`internal/app/services/fhir_spark/`](internal/app/services/fhir_spark/)
- **Technology**: Blaze FHIR Server (Samply/Blaze)
- **Access**: Internal only, proxied through `/fhir` routes
- **Features**:
  - FHIR R4 compliant
  - Patient, Practitioner, Organization management
  - Observation, Appointment, Schedule handling
  - Bundle operations support

### 3. Authentication & Authorization
**Location**: [`internal/app/services/core/auth/`](internal/app/services/core/auth/)
- **Technology**: SuperTokens + Custom RBAC
- **Features**:
  - Magic link authentication
  - Session management via Redis
  - Role-based permissions (Casbin)
  - API key authentication for superadmin

### 4. Payment Gateway Integration
**Location**: [`internal/app/services/shared/payment_gateway/`](internal/app/services/shared/payment_gateway/)
- **Technology**: OY! Indonesia Payment Gateway
- **Services**: analyze, report, performance-report, access-dataset
- **Features**:
  - Service-based pricing
  - Transaction tracking
  - Payment status monitoring

## Request Flow Architecture

### 1. Authentication Flow
```
Request → API Gateway → SuperTokens Middleware → Session Validation → RBAC Check → Service
```

### 2. FHIR Proxy Flow
```
/fhir/* → Auth Middleware → RBAC Filter → Proxy Bridge → Blaze Server → Response Filter
```

### 3. Payment Flow
```
/pay/* → Auth Middleware → RBAC Filter → Payment Service → OY! Gateway → Transaction Storage
```

## Data Architecture

### 1. FHIR Data (Blaze Server)
- **Storage**: Internal Blaze FHIR server
- **Resources**: Patient, Practitioner, Organization, Appointment, Observation, etc.
- **Access**: Proxied through backend with RBAC filtering

### 2. Session Data (Redis)
- **Storage**: Redis cache
- **Data**: User sessions, temporary tokens, rate limiting counters
- **TTL**: Configurable expiration times

## Security Architecture

### 1. Authentication Layers
- **SuperTokens**: Primary authentication system
- **API Keys**: Superadmin access with rate limiting
- **Session Management**: Redis-based session storage

### 2. Authorization (RBAC)
**Configuration**: [`resources/rbac_model.conf`](resources/rbac_model.conf), [`resources/rbac_policy.csv`](resources/rbac_policy.csv)
- **Roles**: Guest, Patient, Practitioner, Clinic Admin, Researcher, Superadmin
- **Permissions**: Method + Path based access control
- **Enforcement**: Casbin policy engine

### 3. Data Filtering
- **FHIR Response Filtering**: Role-based resource filtering in Bundle responses
- **Ownership Validation**: Patient/Practitioner resource ownership checks
- **Query Parameter Filtering**: Automatic filtering based on user context

## Deployment Architecture

### 1. Containerization
- **Base Image**: Go Alpine
- **Multi-stage Build**: Vendor dependencies cached separately
- **Configuration**: Environment variables + YAML config files

### 2. Service Dependencies
```yaml
services:
  - redis-core-konsulin (Sessions, Cache)
  - blaze-core-konsulin (FHIR Server)
  - supertokens-core-konsulin (Auth Service)
```

### 3. Production Setup
- **Domain**: api.konsulin.care
- **SSL**: Let's Encrypt certificates
- **Reverse Proxy**: Nginx with Docker labels
- **Monitoring**: JSON file logging with rotation

## Key Design Patterns

### 1. Clean Architecture
- **Layers**: Delivery → Services → Repositories → External
- **Dependency Injection**: Interface-based dependency management
- **Separation of Concerns**: Clear boundaries between layers

### 2. Repository Pattern
- **Contracts**: [`internal/app/contracts/`](internal/app/contracts/)
- **Implementations**: Service-specific repository implementations
- **Testing**: Mock-friendly interface design

### 3. Middleware Chain
- **Order**: Request ID → Logging → Body Buffer → CORS → SuperTokens → API Key → Session → Rate Limit → Error Handler
- **Modularity**: Each middleware handles specific concerns

### 4. Proxy Pattern
- **FHIR Bridge**: [`internal/app/delivery/http/middlewares/proxy.go`](internal/app/delivery/http/middlewares/proxy.go)
- **Features**: Request transformation, response filtering, compression handling

## Performance Considerations

### 1. Caching Strategy
- **Redis**: Session data, rate limiting counters
- **HTTP Client**: Connection pooling for external services
- **Response Caching**: Conditional caching for public resources

### 2. Connection Management
- **Database**: Connection pooling
- **HTTP Clients**: Reusable clients with timeouts
- **Resource Cleanup**: Proper context cancellation

### 3. Rate Limiting
- **Implementation**: Token bucket algorithm
- **Granularity**: Per-user and per-API-key limits
- **Storage**: Redis-based counters

## Monitoring and Observability

### 1. Logging
- **Technology**: Zap structured logging
- **Levels**: Debug, Info, Warn, Error
- **Context**: Request ID tracking throughout request lifecycle

### 2. Health Checks
- **Cache**: Redis connectivity
- **External Services**: FHIR server availability

### 3. Metrics
- **Request Metrics**: Response times, status codes
- **Business Metrics**: Payment success rates, user activity
- **System Metrics**: Memory usage, goroutine counts