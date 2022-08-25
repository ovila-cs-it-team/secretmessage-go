################################
# STEP 1 build executable binary
################################
FROM golang:alpine AS builder

# Install git.
# Git is required for fetching the dependencies.
RUN apk update && apk add --no-cache git && apk add --no-cache make && apk add --no-cache bash

# Create appuser.
ENV USER=appuser
ENV UID=10001

# See https://stackoverflow.com/a/55757473/12429735RUN
RUN adduser --disabled-password --gecos "" --home "/nonexistent" --shell "/sbin/nologin" --no-create-home  --uid "${UID}" "${USER}"

WORKDIR $GOPATH/cmd/

COPY . .

RUN make build


##############################################
### STEP 2 build a small image from the binary
##############################################
FROM alpine:latest

# Import the user and group files from the builder.
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Copy our static executable.
COPY --from=builder /go/cmd/secretmessage /go/bin/secretmessage

# Use an unprivileged user.
USER appuser:appuser

# Expose the port
EXPOSE 8080

# Run the app binary.
ENTRYPOINT ["/go/bin/secretmessage"]