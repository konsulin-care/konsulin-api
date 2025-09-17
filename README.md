# Konsulin Backend Service

Welcome to the **Konsulin** backend service repository. This service serves as the API Gateway for the Konsulin digital health platform, enhancing mental well-being through self-paced exercises and various psychological tools. This README provides an overview of the service architecture, features, and how to set up the development environment.

## Overview

Konsulin is a digital health platform that offers self-paced exercises to improve mental health. The backend serves as a centralized API Gateway that routes requests to appropriate internal and external services, providing authentication, authorization, and secure access to healthcare data.

## Architecture

The backend aims for a **Clean Architecture** pattern with **API Gateway** design, serving as the entry point for all client applications (frontend, mobile apps, external integrators). Key architectural components:

- **API Gateway**: Central request router with authentication and authorization
- **FHIR Integration**: Blaze FHIR server for healthcare data storage (FHIR R4 compliant)
- **Authentication**: SuperTokens with magic link authentication
- **Authorization**: Role-based access control (RBAC) using Casbin
- **Payment Processing**: OY! Indonesia payment gateway integration
- **Session Management**: Redis-based session storage

## Features

- **Psychological Instruments**: Access to various psychometric tools and assessments
- **Digital Interventions**: Evidence-based exercises for self-compassion, mindfulness, and mental health
- **Appointment Management**: Schedule and manage appointments with psychologists
- **Payment Gateway**: Secure payment processing for healthcare services
- **FHIR-Compliant Health Records**: Comprehensive health record management using FHIR R4 standards
- **Role-Based Access Control**: Fine-grained permissions system with multiple user roles

## Technology Stack

### Core Technologies
- **Language**: Go 1.22.3
- **HTTP Router**: Chi v5
- **Architecture**: Clean Architecture with API Gateway pattern

### Data Storage
- **Primary Data**: Blaze FHIR Server (FHIR R4 compliant)
- **Sessions & Cache**: Redis
- **Authentication Database**: PostgreSQL (SuperTokens only)

### Authentication & Authorization
- **Authentication**: SuperTokens (passwordless magic link)
- **Authorization**: Casbin RBAC
- **Session Management**: Redis-based sessions
- **API Keys**: Custom implementation for superadmin access

### External Integrations
- **Payment Gateway**: OY! Indonesia
- **Messaging**: RabbitMQ (email, WhatsApp notifications)

## Prerequisites

- Go 1.22.3 or later
- Docker & Docker Compose
- Git

## Local Development Setup

### 1. Clone the Repository
```bash
git clone https://github.com/yourusername/be-konsulin.git
cd be-konsulin
```

### 2. Install Dependencies
```bash
go mod tidy
```

### 3. Configure Environment
Create a `.env` file in the root directory using `.env.example` as a template:
```bash
cp .env.example .env
```

**Ask fellow Engineers for .env credentials**

### 4. Start Development Services
Start the required services (PostgreSQL for SuperTokens, Redis, Blaze FHIR server, SuperTokens):
```bash
docker-compose up -d
```

This will start:
- `postgres-core-konsulin`: PostgreSQL database for SuperTokens (port 7500)
- `redis-core-konsulin`: Redis for sessions and caching (port 6379)
- `blaze-core-konsulin`: Blaze FHIR server for healthcare data (port 8080)
- `supertokens-core-konsulin`: SuperTokens authentication service (port 3567)

### 5. Run the Backend Service
```bash
go run cmd/http/main.go
```

The API Gateway will be available at the configured port (default: check your `.env` file).

## API Architecture

### Request Flow
```
Client Request → API Gateway → Authentication → Authorization → Service Routing → Response
```

### Route Patterns
- `/auth/*` - Authentication and user management (SuperTokens)
- `/fhir/*` - FHIR resources (proxied to Blaze server with RBAC filtering)
- `/pay/*` - Payment processing (OY! Indonesia integration)
- `/hook/*` - Webhook handling (internal and external)

### Authentication & Authorization
The system uses SuperTokens for authentication with magic link login. Authorization is handled through Casbin RBAC with the following roles:

- **Guest**: Unauthenticated users with limited access
- **Patient**: Healthcare consumers
- **Practitioner**: Healthcare providers
- **Clinic Admin**: Healthcare facility administrators
- **Researcher**: Data analysts with access to anonymized datasets
- **Superadmin**: System administrators with full access

For detailed role permissions, see [`resources/rbac_policy.csv`](resources/rbac_policy.csv).

## Payment Services

The platform supports service-based pricing through OY! Indonesia payment gateway:

### Available Services
- `analyze`: Patient data analysis (min quantity: 10)
- `report`: Practitioner reports (min quantity: 1)
- `performance-report`: Performance analytics (min quantity: 1)
- `access-dataset`: Research dataset access (min quantity: 1)

### Access Rules
- `analyze` → patient role
- `report` → practitioner role
- `performance-report` → clinic_admin role
- `access-dataset` → researcher role
- All services → superadmin role

### Request Format
```json
{
  "total_item": 3,
  "service": "analyze",
  "body": { 
    "email": "user@email.com",
    "additional_data": "..."
  }
}
```

## Docker Deployment

### Build Vendor Dependencies
```bash
bash build-vendor.sh
```

### Build and Deploy
```bash
# Basic build
bash build.sh -a 'Your Name'

# Complete build with all parameters
bash build.sh -a 'Your Full Name' -e your.email@example.com -v develop
```

Parameters:
- `-a`: Author name
- `-e`: Author email
- `-v`: Deployment version (`develop`, `staging`, or `production`)

### Test Docker Build
```bash
# Comment out ENTRYPOINT in Dockerfile, then:
docker run --rm -it konsulin/api-service:0.0.1 bash
```

## API Documentation

Please see the `/docs` directory for Postman collections and API documentation, or contact the development team for access.

## Health Checks

The service provides health check endpoints for monitoring:
- Redis connectivity
- FHIR server availability
- Service status

## Development Guidelines

### Code Structure
The project follows Clean Architecture principles:
- `cmd/`: Application entry points
- `internal/app/delivery/`: HTTP handlers and middleware
- `internal/app/services/`: Business logic and use cases
- `internal/app/contracts/`: Interface definitions
- `internal/app/drivers/`: External service drivers

### Key Middleware Chain
Request processing follows this middleware order:
1. Request ID generation
2. Structured logging
3. Body buffering
4. CORS handling
5. SuperTokens authentication
6. API key validation
7. Session management
8. Rate limiting
9. Error handling

## Contributing

We welcome contributions from team members. Please follow the established coding standards and architecture patterns.

## License

This project is licensed under the MIT License.

---

Thank you for using Konsulin. For more information, visit our [website](#) or contact us at [email](#).
