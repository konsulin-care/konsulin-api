# Project Authorship 
Author: Abraham Purnomo  
Email: [abrahampurnomo144@gmail.com](mailto:abrahampurnomo144@gmail.com)  
GitHub: [bsr144](https://github.com/bsr144)

# Konsulin Backend Service

Welcome to the **Konsulin** backend service repository. This service is designed to support the Konsulin digital health platform, enhancing well-being through self-paced exercises and various psychological tools. This README provides an overview of the service, its features, and how to use it.

## Overview

Konsulin is a digital health platform that offers self-paced exercises to improve mental health. Key features of the platform include psychological assessments, digital interventions, appointment management, payment processing, and an integrated health record system.

## Features

- **Psychological Instruments**: Access to various psychometric tools and assessments.
- **Digital Interventions**: Evidence-based exercises aimed at improving self-compassion, mindfulness, and overall mental health.
- **Appointment Management**: Schedule and manage appointments with psychologists.
- **Payment Gateway**: Secure and integrated payment processing for services.
- **Integrated Health Records**: Maintain comprehensive health records for users.

## Subscription Tiers

- **Free**: Access to all psychometric instruments, psychologist appointments, and limited psychological exercises.
- **Essential**: Full access to all psychological exercises.
- **Premium**: Includes all Essential features plus discounted psychologist appointments.
- **Elite**: Includes all Premium features plus home care consultations.

## Usage

This backend service is built using Golang and provides RESTful APIs to interact with the Konsulin platform. Below are the steps to set up and run the service.

### Prerequisites

- Go 1.16 or later
- PostgreSQL database
- [Docker](https://www.docker.com/) (optional for containerization)

### Installation

1. **Clone the repository**:
    ```sh
    git clone https://github.com/yourusername/be-konsulin.git
    cd be-konsulin
    ```

2. **Install dependencies**:
    ```sh
    go mod tidy
    ```

3. **Configure environment variables**:
    Create a `.env` file in the root directory with the following variables (or see .env.example):
    ```env
    DB_HOST=your_db_host
    DB_USER=your_db_user
    DB_PASSWORD=your_db_password
    DB_NAME=your_db_name
    JWT_SECRET=your_jwt_secret
    SMTP_SERVER=smtp_server
    SMTP_PORT=smtp_port
    SMTP_USER=smtp_user
    SMTP_PASSWORD=smtp_password
    ```

4. **Ask fellow Engineers for .env credentials**

### Running the Service
1. **Run containers**:
    ```sh
    docker-compose up -d --build
    ```

2. **Start the server**:
    ```sh
    go run cmd/http/main.go
    ```

3. **Docker**:
    Alternatively, you can use Docker to run the service:
    ```sh
    docker build -t konsulin-backend .
    docker run -d -p 8080:8080 --env-file .env konsulin-backend
    ```

### API Endpoints
Please see `/docs` directory to get your Konsulin Postman Collection or contact [CEO](aly.lamuri8@gmail.com) or [Software Engineer](abrahampurnomo144@gmail.com)

## Dockerization

* Build docker image
```shell
bash build-vendor.sh
```
* Run the app with a docker container
  * format
    ```shell
    bash build.sh -a '<author name>' -e <author email> -v <deployment type>
    ```
    * argument `-a` represents the **author's name** 
    * argument `-e` represents the **author's email** 
    * argument `-v` represents the **deployment version**; expected value: `develop`, `staging`, or `production` 
  * command with a compact version
    ```shell
    bash build.sh -a ardi
    ```
  * command with a complete arguments
    ```shell
    bash build.sh -a 'Muhammad Febrian Ardiansyah' -e mfardiansyah.id@gmail.com -v develop
    ```
* Test running docker
  * comment out the last line
    ```shell
    ...
    #ENTRYPOINT ["./api-service"]
    ...
    ```
  * run docker command:
    ```shell
    docker run --rm -it konsulin/api-service:0.0.1 bash
    ```

## Contribution
We welcome contributions! But only if you are part of the team >_< .

## License
This project is licensed under the MIT License.

---

Thank you for using Konsulin. For more information, visit our [website](#) or contact us at [email](#).
