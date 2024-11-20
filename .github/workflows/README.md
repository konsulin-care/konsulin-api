# BE-KONSULIN DEPLOYMENT METHOD

## Development Workflow

The  GitHub Actions workflow automates the process of containerizing and deploying a project on the `develop` branch. It can be triggered manually or by a push to the `develop` branch.

## Production Workflow

### Step 1: Create Release

This step is to create a release on the GitHub repository, when a new release is created, the `prod-build.yml` workflow will run a Docker build process to build and tag the release image directly on the server.

Step to create a release:

1. Go to the GitHub repository.
2. Click on the "Releases" tab.
3. Click on the "Draft a new release" button.
4. Enter the release tag name. Example: `v1.0.0`.
5. Name the release. Example: `v1.0.0`.
6. Ensure the release tag and release name are same.
7. Optionally, you can generate a release note automatically by clicking on the "Generate release notes" button.
8. Set the target branch to `develop` or trunk branch you want to deploy.
9. Set the release as a `Set as the latest release` option.
10. Click on the "Publish release" button.

### Rules

1. The allowed format for the release tag is `v1.0.0`.
2. The allowed format for the release name is `v1.0.0`.
3. Release name and release tag must be same.

### Example of Correct Release Name

- `v1.0.0`
- `v1.0.0-beta.1`
- `v1.0.0-beta.1+build.123`
- `release-v1.0.0`

### Example of Incorrect Release Name

- `Release v1.0.0`
- `v1.0.0 Release`

### Step 2: Deploy a Release

This step is to deploy a release on the server, when a new release is created, the `prod-release.yml` workflow will run a deployment process.

Step to deploy a release:

1. Go to the GitHub Actions page.
2. Go to `Deploy Production` or `prod-release.yml` workflow.
3. Click on the `Run workflow` button.
4. Fill in the `RELEASE_NAME` input with the release name you want to deploy. See the example of correct release name above [here](#example-of-correct-release-name).
5. Click on the `Run workflow` button.

## WORKFLOW

1. **Containerization (Docker) on Self-Hosted Runner**
   - **Uses**: `docker-self-hosted.yml` workflow.
   - **Parameters**:
     - `TZ_ARG`, `AUTHOR`, `VERSION`, `GIT_COMMIT`, `BUILD_TIME`, `RUN_NUMBER`, `RELEASE_TAG`, `DOCKER_TAG`, `DOCKER_VENDOR_TAG`.
   - **Purpose**: Builds a Docker image directly on the server.

2. **Deployment**
   - **Uses**: `deploy-ansible.yml` workflow.
   - **Parameters**:
     - `DOCKER_TAG`, `ANSIBLE_PLAYBOOK`, `ANSIBLE_INVENTORY_HOSTS`.
   - **Secrets**: `SSH_KEY` to be used by Ansible, ANS
   - **Purpose**: Deploys the Docker container to a remote server.

This workflow streamlines the process of building and deploying code changes to a development environment.

## Containerization (Docker) on Self-Hosted Runner

The "Docker (self-hosted)" GitHub Actions workflow automates the process of building the Docker image directly on the server, which at the same time working as the Self-Hosted Runner. The workflow is manifest file is `.github/workflows/docker-self-hosted.yml`.

### Input Parameters

To re-use the workflow, these are parameters needs to be defined:

- `TZ_ARG`: Timezone setting (default is Asia/Jakarta). This parameter is used to set the timezone for the container and passed into the Docker build process as `--build-arg`.
- `AUTHOR`: Name of the commit author. This parameter is used to set the author for the container and passed into the Docker build process as `--build-arg`.
- `VERSION`: Version of the build. This parameter is used to set the version for the container and passed into the Docker build process as `--build-arg`.
- `GIT_COMMIT`: The Git commit hash. This parameter is used to set the commit hash for the container and passed into the Docker build process as `--build-arg`.
- `BUILD_TIME`: The time the build was created. This parameter is used to set the build time for the container and passed into the Docker build process as `--build-arg`.
- `RUN_NUMBER`: The workflow run number. This parameter is used to set the run number for the container and passed into the Docker build process as `--build-arg`.
- `RELEASE_TAG`: The release tag. The release tag is used to set the release tag for the container and passed into the Docker build process as `--build-arg`.
- `DOCKER_TAG`: The Docker tag. This is the Docker image tag that will be used to tag built image inside the server. This will respectively build the `Dockerfile` file.
- `DOCKER_VENDOR_TAG`: The Docker vendor tag. This is the Docker image tag that will be used to tag built image of Vendor, or we can say, the vendor image that be the base image for the application image. This will respectively build the `Dockerfile-vendor` file.

### Workflow Steps

1. **Prepare:** The workflow will clone the repository to the server.
2. **Build Vendor Image:** The workflow will build the vendor image using the `Dockerfile-vendor` file. It will take the input from DOCKER_VENDOR_TAG and build the image with the tag. The Vendor build image step enabling `DOCKER_BUILDKIT=1` to enhance cahcing and reduce the build time.
3. **Update Dockerfile Base Image Tag:** The workflow will update the `Dockerfile` file with the built result from **Build Vendor Image** step. It will respectively update the base image with the built of Vendor image that tagged with the input from DOCKER_VENDOR_TAG.
4. **Build Main Image:** The workflow will build the main application image using the `Dockerfile` file. It will take the input from DOCKER_TAG and build the image with the tag.

## Deployment (Ansible)

The "Deploy (Ansible)" GitHub Actions workflow automates the deployment of a service to a remote server using Ansible. The workflow is manifest file is `.github/workflows/deploy-ansible.yml`.

### Input Parameters

To re-use the workflow, these are parameters needs to be defined:

- `DOCKER_TAG`: The Docker tag. This is the Docker image tag that will be used to tag built image inside the server.
- `ANSIBLE_PLAYBOOK`: The Ansible playbook file.
- `ANSIBLE_INVENTORY_HOSTS`: The Ansible inventory hosts.
- `SSH_KEY`: The SSH key to be used by Ansible.

### Workflow Steps

1. **Prepare:** The workflow will prepare the environment by cloning the repository to the server.
2. **Run playbook:** The workflow will run the Ansible playbook using the input from `ANSIBLE_PLAYBOOK` and `ANSIBLE_INVENTORY_HOSTS` to deploy the service.  On the playbook we define a variable with name `image_tag` with null value. The Ansible will replace the null value with the input from `DOCKER_TAG`. See Ansible official documentation for more details [Using Variables](https://docs.ansible.com/ansible/latest/user_guide/playbooks_variables.html#using-variables).

### Managing Ansible Variables

Refer to this [Deployment Scripts Documentation](https://github.com/konsulin-care/be-konsulin/blob/develop/deployments/README.md) file for more information on how to manage Ansible variables.

## Deprecated

<details>
   <summary>Containerization (Docker) (Nexus Image Registry)</summary>

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
</details>

<details>
   <summary>Deployment (Docker Compose of Related Service on IaC Repository)</summary>
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
</details>
