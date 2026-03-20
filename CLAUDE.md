# Habitual

A gamified daily habit tracker. Users define habits (e.g. "read for 30 minutes", "do 2 TryHackMe tasks", "write code for 1 hour on project X") and earn points/streaks for completing them each day.

## Tech Stack

- **Backend**: Go
- **Templating**: [templ](https://templ.guide/) — Go-based type-safe HTML components
- **Interactivity**: [HTMX](https://htmx.org/) — hypermedia-driven UI, no JS framework
- **Database**: PostgreSQL (via `pgx/v5`)
- **CSS**: Plain CSS or Tailwind (TBD)
- **DB Driver**: `pgx/v5` with `pgxpool` for connection pooling

## Project Structure

```
habitual/
├── cmd/
│   └── habitual/
│       └── main.go          # Entry point
├── internal/
│   ├── db/                  # Database layer (queries, migrations)
│   ├── handler/             # HTTP handlers
│   ├── model/               # Domain types (Habit, Entry, etc.)
│   └── service/             # Business logic
├── web/
│   ├── components/          # templ components
│   └── static/              # CSS, JS (htmx), images
├── migrations/              # SQL migration files
├── CLAUDE.md
└── go.mod
```

## Core Concepts

- **Habit**: A recurring task with a name, description, target frequency (daily), and point value
- **Entry**: A log of a completed habit on a specific date
- **Streak**: Consecutive days a habit has been completed
- **Points**: Earned per habit completion; can track totals per day/week/all-time

## Development Commands

```bash
# Generate templ files
templ generate

# Run the server
go run ./cmd/habitual

# Run tests
go test ./...
```

## Conventions

- Use `templ` components for all HTML — no `html/template`
- HTMX for partial page updates (avoid full-page reloads where sensible)
- Keep handlers thin — business logic lives in `internal/service`
- SQL queries live in `internal/db`, no ORM
- Migrations are numbered SQL files in `migrations/`
