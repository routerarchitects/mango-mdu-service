# Mango MDU Service

MDU orchestration service for Router Architects.

## Purpose

This service will act as the backend orchestrator for the MDU UI and coordinate calls across provisioning, security, topology, gateway, and related platform services.

## Stack

- Go
- Fiber v3
- Router common modules
- Service discovery
- Service RPC
- OpenAPI-first contracts

## Development

```bash
go mod tidy
go test ./...
go run ./cmd