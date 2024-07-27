# Build stage
FROM golang:1.22-alpine AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /checkboxes ./cmd/checkboxes/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /client ./cmd/client/

# Final stage
FROM scratch

# Copy the Pre-built binary file from the previous stage
COPY --from=builder /checkboxes /checkboxes
COPY --from=builder /client /client

# Command to run the executable
ENTRYPOINT ["/checkboxes"]

# Accept arguments for relayer or master mode
CMD ["-mode", "master"]
