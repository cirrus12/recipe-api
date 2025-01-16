# Use official Go image as base
FROM golang:1.21-alpine

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum (if you have them)
COPY go.mod ./

# Copy source code
COPY . .

# Build the application
RUN go build -o main .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]