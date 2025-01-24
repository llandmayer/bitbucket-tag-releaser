FROM golang:1.22.4-alpine3.20

RUN apk add --no-cache python3 pipx 

ENV PATH="/root/.local/bin:$PATH"

RUN pipx install keepachangelog-manager

WORKDIR /usr/src/app

# pre-copy/cache go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o /usr/local/bin/app ./...

CMD ["app"]