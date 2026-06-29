---
layout: home

title: EgoAdmin
titleTemplate: Go Microservice Admin Template Based on EGO

hero:
  name: EgoAdmin
  text: Enterprise Go Microservice Admin Template
  tagline: gateway + user + idgen · Embedded Vue 3 + Element Plus frontend · Proto-first APIs · Atlas migrations · DTM distributed transactions
  actions:
    - theme: brand
      text: Getting Started
      link: /en-US/guide/getting-started
    - theme: alt
      text: GitHub
      link: https://github.com/egoadmin/egoadmin

features:
  - icon: 🏗️
    title: Three-Service Architecture
    details: gateway for external entry and embedded frontend, user for identity and permissions, idgen for Snowflake IDs and leases.
  - icon: 📋
    title: Proto-First API Contracts
    details: Business APIs start from .proto files. HTTP endpoints are gRPC compatibility mappings using POST body requests.
  - icon: 🎨
    title: Embedded Vue 3 Admin UI
    details: The web/ app uses Vue 3, Element Plus, Pinia and Vue Router. Its dist output is embedded into gateway with go:embed.
  - icon: 🔐
    title: Closed Permission Chain
    details: authsession Bearer auth, Casbin API permissions, DataScope, routeMenu and permission-contract work as one closed chain.
---

## Recommended Reading

1. [Getting Started](/en-US/guide/getting-started)
2. [Feature Overview](/en-US/guide/features)
3. [Microservice Architecture](/en-US/guide/architecture)
4. [API Development Workflow](/en-US/guide/api-development)

## Quick Start

```bash
git clone https://github.com/egoadmin/egoadmin.git
cd egoadmin
export DOCKER_REGISTRY=ghcr.io/egoadmin
make deploy-up
```

Open `http://localhost:9001` and sign in with `admin` / `123456`.
