# Inventory & Order Management System

A RESTful API for inventory and order management built with **Go**, **Gin**, and **PostgreSQL**.

## Features

- **User Authentication** — JWT-based registration & login with role-based access (admin / customer)
- **Product Management** — CRUD operations with categories, filtering, pagination, and sorting
- **Inventory Tracking** — Stock levels, reservations, restock operations, and full audit trail
- **Order Processing** — Order lifecycle management with status transitions and stock validation
- **Database Migrations** — Versioned schema migrations via golang-migrate

## Tech Stack

| Layer       | Technology                  |
|-------------|-----------------------------|
| Language    | Go 1.22+                   |
| Framework   | Gin                        |
| Database    | PostgreSQL 16               |
| DB Driver   | pgx/v5 with connection pool |
| Auth        | JWT (golang-jwt/v5)         |
| Migrations  | golang-migrate/v4           |
| Testing     | testify                     |
| Containers  | Docker & Docker Compose     |

## Project Structure

```
├── cmd/server/             # Application entrypoint
├── internal/
│   ├── config/             # Environment-based configuration
│   ├── database/           # PostgreSQL connection pool
│   ├── handlers/           # HTTP request handlers
│   ├── middleware/          # Auth & logging middleware
│   ├── models/             # Domain types & request/response structs
│   ├── repository/         # Database access layer
│   ├── router/             # Route definitions
│   └── services/           # Business logic
├── migrations/             # SQL migration files
├── docker-compose.yml      # PostgreSQL + app services
├── Dockerfile              # Multi-stage build
├── Makefile                # Common dev commands
└── .env.example            # Environment variable template
```

## Getting Started

### Prerequisites

- Go 1.22+
- PostgreSQL 16+ (or Docker)
- [golang-migrate CLI](https://github.com/golang-migrate/migrate) (optional, for manual migrations)

### 1. Clone & configure

```bash
cp .env.example .env
# Edit .env with your database credentials and JWT secret
```

### 2. Start the database

```bash
make docker-up
```

Or use an existing PostgreSQL instance and update `.env` accordingly.

### 3. Run migrations

```bash
make migrate-up
```

### 4. Run the server

```bash
make run
```

The API will be available at `http://localhost:8080`.

## Available Commands

```bash
make build          # Compile binary to bin/server
make run            # Run the server with go run
make test           # Run all tests with race detection & coverage
make migrate-up     # Apply database migrations
make migrate-down   # Rollback database migrations
make docker-up      # Start services with Docker Compose
make docker-down    # Stop Docker Compose services
make lint           # Run golangci-lint
```

## Database Schema

The system uses six tables with referential integrity and constraints:

- **users** — accounts with email/password and role
- **categories** — product groupings
- **products** — items with price, SKU, and category
- **inventory** — stock levels and reservations per product
- **inventory_transactions** — audit trail for all stock changes
- **orders / order_items** — customer orders with line items

## Environment Variables

| Variable       | Default                              | Description              |
|----------------|--------------------------------------|--------------------------|
| `DB_HOST`      | `localhost`                          | PostgreSQL host          |
| `DB_PORT`      | `5432`                               | PostgreSQL port          |
| `DB_NAME`      | `order_management`                   | Database name            |
| `DB_USER`      | `postgres`                           | Database user            |
| `DB_PASSWORD`  | `postgres`                           | Database password        |
| `DB_SSLMODE`   | `disable`                            | SSL mode                 |
| `JWT_SECRET`   | `your-secret-key-change-in-production` | JWT signing key        |
| `SERVER_PORT`  | `8080`                               | HTTP server port         |

## License

This project is for educational and internal use.
