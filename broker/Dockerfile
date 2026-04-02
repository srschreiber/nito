# Copyright 2026 Sam Schreiber
#
# This file is part of nito.
#
# nito is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# nito is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with nito. If not, see <https://www.gnu.org/licenses/>.

FROM golang:latest AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /broker ./broker

FROM alpine:3
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /broker /broker
ENTRYPOINT ["/broker"]