# Nina Test App

A simple Go HTTP server built with Gin that returns "hello world" as JSON.

## Features

- Returns HTTP 200 with "hello world" message as JSON on the root endpoint (`/`)
- Runs on port 8080 by default
- Configurable port via `PORT` environment variable
- Built with the Gin web framework

## Prerequisites

- Go 1.21 or later

## Running the Server

### Using default port (8080)

```bash
go run main.go
```

### Using custom port

```bash
PORT=3000 go run main.go
```

Or on Windows:

```bash
set PORT=3000 && go run main.go
```

## Testing the Server

Once the server is running, you can test it with curl:

```bash
curl http://localhost:8080/
```

Expected response:
```json
{"message":"hello world"}
```

## Building

To build the executable:

```bash
go build -o nina-test-app main.go
```

Then run it:

```bash
./nina-test-app
``` 