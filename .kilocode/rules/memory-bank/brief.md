# Backend as the API Gateway

The backend serves as the centralized API gateway for the application. It is responsible for receiving and routing incoming HTTP requests to the appropriate internal or external services based on the URL path. This architecture ensures a clean separation of responsibilities while maintaining a consistent entry point for client applications (frontend, mobile apps, external integrators).

# Purpose of the Backend

The backend is not responsible for performing core domain logic, but rather acts as a request router, authenticator, and access controller. Its responsibilities include:
- Authenticating users via SuperTokens.
- Enforcing role-based access controls.
- Routing requests to internal services such as Blaze (FHIR) and webhooks.
- Forwarding requests to external services such as OY! Indonesia (payment gateway).
- Providing secure API key-based access for privileged integrations (e.g., superadmin).

# Request Routing Behavior

All incoming requests first pass through authentication (via SuperTokens) and role enforcement, except those explicitly marked as public (e.g., login, registration, guest access). The guest role is the default for unauthenticated users and provides access to a limited set of routes defined in the authorization schema.

Additionally, for high-privilege operations (e.g., integration management), API key-based access is implemented for the superadmin role. This key must be securely provided via environment variables.

# System Architecture

The backend is designed as a gateway layer, implemented as a stateless service that intermediates communication between clients and service-specific components. It follows a modular architecture, where each route prefix (/fhir, /auth, /pay, /hook) maps to a dedicated service, either internal or external.

High-level architecture:
- API Gateway (Backend Core): Accepts all HTTP requests, performs authentication, validates access based on role/API key, and routes the request accordingly.
- FHIR Service (Blaze): Handles all requests under /fhir. Deployed internally and not exposed to the internet.
- Auth Service (SuperTokens): Handles all authentication and authorization logic via /auth.
- Payment Gateway (OY! Indonesia): Used under /pay. A proxy layer forwards requests to OY's public APIs.
- Webhook Service: Handles incoming and outgoing webhook requests under /hook. Partially internal, with selective external exposure.

Each service is containerized and deployed independently. Production-ready FHIR instance is connected via an internal Docker network.
