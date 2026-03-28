# Scalable Chat System (WebSocket) — Main Service

This service is the core chat gateway for our scalable real-time chat system. It is written in Go and is designed to run as a containerized service on AWS ECS. The service supports:

- WebSocket connections for real-time communication
- Redis-based user presence tracking with TTL and heartbeat refresh
- Cross-instance message forwarding over HTTP
- DynamoDB-based offline message fallback for reliability

The goal of this service is to support 1:1 messaging and group chat under scalable distributed deployment. Multiple chat gateway tasks can run simultaneously, each accepting WebSocket connections, storing online user presence in Redis, and forwarding messages either locally or to another task. If real-time delivery fails or a target user is offline, the message is stored in DynamoDB and replayed when the user reconnects.

---

## Features

- WebSocket-based client connections for real-time chat
- Local in-memory map of connected users per task
- Redis presence tracking with TTL — stale mappings are automatically removed on task/node failure
- Periodic heartbeat to refresh online user presence
- Cross-instance message forwarding via internal HTTP endpoint
- DynamoDB storage of undelivered messages for offline users
- Offline message replay on user reconnect

---

## High-Level Architecture

Clients connect through a WebSocket endpoint exposed by the chat gateway. The gateway runs as one or more ECS tasks behind an Application Load Balancer. When a user connects, the task records their presence in Redis. When a message is sent:

1. The service checks Redis to locate the target user.
2. If the target is on the **same task** → deliver locally via WebSocket.
3. If the target is on a **different task** → forward via internal HTTP.
4. If the target is **offline or presence is stale** → write to DynamoDB.

When the user reconnects, offline messages are loaded from DynamoDB, replayed, and deleted after successful delivery.

---

## Main Endpoints

| Endpoint | Description |
|---|---|
| `GET /health` | Health check for load balancer integration |
| `GET /ws?user_id=<USER_ID>` | WebSocket connection for clients |
| `POST /internal/send` | Internal task-to-task message forwarding (not exposed externally) |

---

## Message Flow

### Connection

When a user connects via WebSocket, the current task:
1. Adds the user to its local connection map.
2. Writes a Redis presence entry with a TTL.
3. Starts a heartbeat to keep the presence valid.

### Sending a Message

1. Determine the members of the target conversation.
2. For each target user, check Redis presence.
3. Deliver locally, forward to another task, or write to DynamoDB as appropriate.

### Reconnect

1. Load offline messages from DynamoDB.
2. Replay them via the local WebSocket connection.
3. Delete the messages from DynamoDB after successful replay.

---

## Reliability Strategy

This service uses **best-effort real-time delivery** combined with **durable offline fallback**.

- Redis presence is used as a **routing hint**, not a guaranteed delivery source.
- TTL on Redis entries ensures stale presence is removed automatically after failures.
- Heartbeat keeps valid online users active in Redis.
- If a task fails, its users' presence entries expire; reconnecting users get new presence on the new task.
- Undeliverable messages are stored in DynamoDB and replayed later.

> This design avoids message loss during task/node failures. The trade-off is **at-least-once delivery** behavior — clients should use `message_id` for deduplication.

---

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `PORT` | Port for the Go service | `8080` |
| `INSTANCE_ADDR` | Routable address of this task, e.g. `http://10.0.1.25:8080` | Auto-detected |
| `REDIS_ADDR` | Redis server address, e.g. `10.0.2.10:6379` | — |
| `REDIS_PASSWORD` | Redis password (if configured) | — |
| `AWS_REGION` | AWS region for DynamoDB access | — |
| `DDB_TABLE_NAME` | DynamoDB table for offline message storage | — |

---

## Redis Usage

Redis stores user presence. Each user is mapped to the current task address, with a TTL so stale entries are removed automatically on task failure. A heartbeat process refreshes all locally connected users at a fixed interval.

**Presence key format:**

```
presence:user:<userID>
```

**Value:** current task address (e.g. `http://10.0.1.25:8080`)

> Redis should run on a separate EC2 instance or a managed Redis-compatible service. ECS tasks must be able to reach it over the VPC network.

---

## DynamoDB Usage

DynamoDB stores offline messages to prevent message loss when a target user is offline or cross-instance forwarding fails.

**Table design:**

| Key | Type | Description |
|---|---|---|
| `receiver_id` | Partition key | Target user ID |
| `sort_key` | Sort key | Timestamp + message ID (for ordered replay) |

Each offline message includes: `receiver_id`, `message_id`, `sender_id`, `conversation_id`, `content`, `message_type`, `timestamp`, `created_at`, and the offline reason.

---

## Running Locally

```bash
export PORT=8080
export REDIS_ADDR=localhost:6379
export AWS_REGION=us-west-2
export DDB_TABLE_NAME=chat-offline-messages

go run main.go
```

Connect a client to:

```
ws://localhost:8080/ws?user_id=user1
```

**Example WebSocket message (JSON):**

```json
{
  "type": "chat",
  "conversation_id": "room-1",
  "content": "hello everyone",
  "message_type": "text"
}
```

---

## Docker

**Build:**

```bash
docker build -t scalable-chat-gateway .
```

**Run locally:**

```bash
docker run -p 8080:8080 \
  -e PORT=8080 \
  -e REDIS_ADDR=host.docker.internal:6379 \
  -e AWS_REGION=us-west-2 \
  -e DDB_TABLE_NAME=chat-offline-messages \
  scalable-chat-gateway
```

---

## AWS Deployment Plan

1. **ECR** — Build and push the Docker image to Amazon ECR.
2. **ECS** — Create a task definition and run one or more tasks from the image.
3. **ALB** — Place the ECS service behind an Application Load Balancer so clients always connect through a stable endpoint. The ALB is critical for WebSocket reconnection after task failure and for horizontal scaling.
4. **Redis** — Deploy Redis on a dedicated EC2 instance (or managed service). ECS tasks must be able to reach it over the VPC; configure security groups accordingly.
5. **DynamoDB** — Create the offline message table with the partition/sort key design above. Grant the ECS task role read/write permissions on this table.

> Recommended tooling: **Terraform** for VPC, subnets, security groups, ALB, ECS cluster, and ECR repository provisioning.

---

## Scalability Experiments

| Experiment | How to Test |
|---|---|
| Connection scaling | Increase concurrent WebSocket clients, keep message rate low |
| Throughput scaling | Fix connection count, increase messages per second |
| Group fan-out | Increase users per room, measure delivery performance |
| Failure recovery | Stop one ECS task under load; observe presence expiry, reconnect behavior, and DynamoDB fallback |

Use **CloudWatch metrics** to observe CPU, memory, healthy target count, ECS task count, and ALB behavior. These experiments show when a single task becomes a bottleneck and when horizontal scaling is needed.

---

## Notes and Current Simplifications

- **Conversation membership** is currently an in-memory mock map. In production this should be backed by DynamoDB, Redis, or a dedicated service.
- **Authentication** is not implemented in this version. User IDs are provided directly at connection time for simplicity.
- This service is best understood as a **distributed real-time chat gateway prototype** designed for scalability experiments, not a full production messaging product.

---

## Suggested Team Responsibilities

| Area | Responsibility |
|---|---|
| Infrastructure | Use Terraform to provision VPC, subnets, security groups, ALB, ECS cluster, ECR repo |
| Deployment | Build and push Docker image; create ECS task definition; attach service to ALB |
| Redis | Deploy Redis on dedicated EC2; configure VPC security groups for ECS ↔ Redis access |
| DynamoDB | Create offline message table; grant ECS task role read/write permissions |
| Testing | Write load generation scripts for WebSocket traffic; run locally then against AWS; collect CloudWatch metrics as experiment evidence |
