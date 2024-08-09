# BE-KONSULIN DEPLOYMENT METHOD

**Production Workflow**

The "Production" GitHub Actions workflow automates the process of building and deploying a project to the production environment. It is triggered by either a manual dispatch or a push to any tagged commit.

**Development Workflow**

The  GitHub Actions workflow automates the process of containerizing and deploying a project on the `develop` branch. It can be triggered manually or by a push to the `develop` branch.

## WORKFLOW

1. **Containerization (Docker)**
   - **Uses**: `docker.yml` workflow.
   - **Parameters**:
     - Timezone, author, version, commit SHA, tag, run number, and build time.
   - **Secrets**: Docker credentials.
   - **Purpose**: Builds a Docker image with the provided build arguments and credentials.

2. **Deployment**
   - **Uses**: `deploy.yml` workflow.
   - **Depends on**: Successful completion of the Docker job.
   - **Parameters**:
     - Environment and service name (`dev-api`) or  (`prod-api`)
   - **Secrets**: SSH and Docker credentials.
   - **Purpose**: Deploys the Docker container to a remote server.

This workflow streamlines the process of building and deploying code changes to a development environment.

## Containerization (Docker)

The "Docker" GitHub Actions workflow automates the process of building and pushing Docker images for a project.

### Inputs

- **TZ_ARG**: Timezone setting (default is Asia/Jakarta).
- **AUTHOR**: Name of the commit author.
- **VERSION**: Version of the build.
- **TAG**: Git tag associated with the build.
- **GIT_COMMIT**: The Git commit hash.
- **BUILD_TIME**: The time the build was created.
- **RUN_NUMBER**: The workflow run number.

### Secrets

- **DOCKER_USERNAME** and **DOCKER_PASSWORD**: Credentials for logging into the Docker registry.

### Jobs

#### Docker Job

- **Runs on**: `ubuntu-latest`

- **Steps**:

  1. **Prepare**:
     - Uses `actions/checkout@v2` to check out the code from the repository.

  2. **Login to Registry**:
     - Uses `docker/login-action@v1` to log into the Docker registry using provided credentials.

  3. **Get SHA Short**:
     - Extracts the first 8 characters of the Git commit SHA to create a short SHA, stored in the environment variable `SHORT_SHA`.

  4. **Get Branch**:
     - Extracts the branch name from the Git reference and stores it in the environment variable `BRANCH`.

  5. **Change Vendor Tags**:
     - Updates the `Dockerfile` to use a specific vendor image tag based on the branch and short SHA.

  6. **Build Vendor Image**:
     - Builds a vendor Docker image with a unique tag and pushes it to the Docker registry.

  7. **Push Vendor Image**:
     - Pushes the vendor Docker image to the specified registry with the tag `sha-${{ env.BRANCH }}-${{ env.SHORT_SHA }}-vendor`.

  8. **Build App Image**:
     - Builds the application Docker image, passing in various build arguments, and tags it with a unique identifier based on the branch and short SHA.

  9. **Push App Image**:
     - Pushes the application Docker image to the Docker registry with the tag `sha-${{ env.BRANCH }}-${{ env.SHORT_SHA }}`.

This workflow streamlines the Docker image creation and deployment process by automating the build, tagging, and pushing steps for both vendor and application images.

## Deployment

The "Deploy" GitHub Actions workflow automates the deployment of a service to a remote server using SSH and Docker.

### Inputs

- **ENVIRONMENT**: Specifies the deployment environment (e.g., development, production).
- **SERVICE_NAME**: The name of the service to be deployed.

### Secrets

- **SSH_HOST**, **SSH_USERNAME**, **SSH_KEY**, **SSH_PORT**: Credentials and details required to connect to the remote server via SSH.
- **DOCKER_USERNAME**, **DOCKER_PASSWORD**: Credentials for logging into the Docker registry.

### Jobs

#### Deployment Job

- **Runs on**: `ubuntu-latest`

- **Steps**:

  1. **Get SHA Short**:
     - Extracts the first 8 characters of the Git commit SHA to create a short SHA, which is stored in the environment variable `SHORT_SHA`.

  2. **Get Branch**:
     - Extracts the branch name from the Git reference and stores it in the environment variable `BRANCH`.

  3. **Executing Remote SSH Commands**:
     - Uses the `appleboy/ssh-action` to connect to the remote server using SSH.
     - Navigates to the appropriate directory for the specified environment.
     - Logs into the Docker registry using the provided credentials.
     - Pulls the latest Docker image for the specified service using a unique commit hash (`COMMIT_HASH`).
     - Deploys the service using Docker Compose to ensure it is updated with the latest version.

This workflow facilitates seamless deployment by automating the steps necessary to securely connect to a remote server, pull the latest Docker images, and deploy services, ensuring that the application is up-to-date with the latest code changes.

