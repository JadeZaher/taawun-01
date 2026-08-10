# Taawun - Source Code

Architecture
cmd/: Application entry point
pkg/: Core business logic (database, handlers, models, repositories, services)
web/: Embedded frontend GUI (compiled directly into the binary)

Requirements
- Go 1.21 or higher
- GCC (required for SQLite CGO compilation)

How to Run (Development)

Mac/Linux:
make run

Windows PowerShell:
1="1"
go run ./cmd/main.go

How to Build for Production

1="1"
go build -ldflags="-s -w" -o Taawun.exe ./cmd/main.go
