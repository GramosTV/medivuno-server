# Healthcare App Server (Golang)

[Client Repository](https://github.com/GramosTV/medivuno-client)

This is the backend server for the Healthcare App, built with Golang and Gin framework. It provides a RESTful API for managing users (patients, doctors, admins), appointments, medical records, and secure messaging.

## Features

- User authentication (Register, Login, Logout, Token Refresh) with JWT.
- Role-based access control (Patient, Doctor, Admin).
- User profile management.
- Appointment scheduling and management.
- Medical record creation, retrieval, updates, and deletion.
- Medical record attachment uploads (stored in the database as binary data) and downloads.
- Secure messaging between users.
- Database interactions via GORM with MySQL.

## Prerequisites

- Go (version 1.20 or higher recommended)
- MySQL
- Git

## Setup

1.  **Clone the repository:**

    ```bash
    git clone <repository-url>
    cd healthcare-app-server-golang
    ```

2.  **Database Setup:**

    - Ensure MySQL is running.
    - Create a database (e.g., `medi`).
    - The server uses GORM's auto-migration feature, so tables will be created automatically on the first run if they don't exist.

3.  **Environment Variables:**

    - Create a `.env` file in the root of the `healthcare-app-server-golang` directory.
    - Copy the contents of `.env.example` into `.env` and update the values accordingly.
      ```bash
      cp .env.example .env
      ```
    - Key variables to configure:
      - `PORT`: Port the server will run on (e.g., 3001).
      - `DB_HOST`: MySQL host (e.g., `localhost`).
      - `DB_PORT`: MySQL port (e.g., `3306`).
      - `DB_USERNAME`: MySQL username.
      - `DB_PASSWORD`: MySQL password.
      - `DB_NAME`: MySQL database name.
      - `JWT_SECRET`: Secret key for signing JWT access tokens.
      - `JWT_REFRESH_SECRET`: Secret key for signing JWT refresh tokens.
      - `ORIGIN`: CORS origin allowed (e.g., `http://localhost:4200` for the Angular client).

4.  **Install Dependencies:**

    ```bash
    go mod tidy
    ```

5.  **Run the Server:**
    ```bash
    go run main.go
    ```
    The server should now be running on the port specified in your `.env` file (default is 3001).

## API Endpoints

Refer to the `internal/routes/routes.go` file for a detailed list of API endpoints and their handlers. Key groups include:

- `/api/v1/auth/...` (Authentication)
- `/api/v1/users/...` (User Management)
- `/api/v1/appointments/...` (Appointments)
- `/api/v1/medical-records/...` (Medical Records & Attachments)
- `/api/v1/messages/...` (Messaging)

## Project Structure

- `cmd/`: Main application entry point (if any specific commands are needed).
- `internal/`: Contains the core application logic.
  - `config/`: Configuration loading.
  - `handlers/`: HTTP request handlers (controllers).
  - `middleware/`: Custom middleware (e.g., authentication, authorization).
  - `models/`: Database models and GORM setup.
  - `routes/`: API route definitions.
  - `services/`: Business logic services (if separated from handlers).
  - `utils/`: Utility functions (e.g., JWT generation, response formatting).
- `main.go`: Main application entry point.
- `go.mod`, `go.sum`: Go module files.
- `.env.example`: Example environment file.
- `README.md`: This file.

## Contributing

Please refer to the main project's contributing guidelines.
