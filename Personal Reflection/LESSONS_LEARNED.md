# Lessons Learned — Distributed Chatroom Project

**Author:** Liecheng Jiang
**Course:** CS6650 — Final Project

A personal reflection on what I learned building this project from scratch. Organized roughly in the order I learned them, with some notes on what I got wrong, why, and what I'd do differently next time.

On our team, I was primarily responsible for the backend code — designing the database schema, writing the protobuf service contracts, and implementing all four services (Gateway, Message Transmit, Chat Service, Message Storage). I also worked on the Terraform deployment alongside my teammates. The reflections below focus on the parts I personally worked on.

---

## 1. How to actually structure a distributed backend

Before this project, I knew the words — microservices, gRPC, database, cache — but I had never sat down and designed a system from zero. I learned that a "mature" distributed backend comes together in a very specific order, and skipping any step makes the later ones harder.

### Start with the data model

The first thing that genuinely surprised me was how much of the project was decided at the database-schema stage, before I wrote a single line of Go. I designed five tables — `users`, `relations`, `chat_sessions`, `chat_session_members`, and `messages` — and every single service decision downstream was shaped by those choices.

The most important one was putting `last_seq` on the `chat_sessions` row instead of trying to compute it from the `messages` table on the fly. This meant I could allocate a new `session_seq` atomically inside a single MySQL transaction: insert into `messages`, update `chat_sessions.last_seq`. If I had made `session_seq` a computed value, I would have hit race conditions the first time two users sent a message at the same time — and I probably wouldn't have noticed until the experiment phase, when it would have been too late to fix cleanly.

**Lesson:** In a distributed system, the schema is the contract. Get it right before you write application code.

### Object definitions are trickier than they look

The second surprise was how much time I spent on the protobuf message definitions. Naming fields, deciding what's required versus optional, picking between `uint64` and `int64`, figuring out how to represent `null` vs "zero" — all of this sounds trivial until you realize these definitions are the only thing that keeps four independent services honest with each other.

I had to redefine `ChatSession` twice. The first version conflated the "direct vs group" type distinction with member counts. The second version handled it cleanly with an enum. This kind of rework is cheap when you're editing a `.proto` file; it would have been painful if I had already hand-written the types in Go.

**Course concept:** this is the classic tradeoff between *schema evolution* and *consistency across services*. Using a single source of truth like protobuf makes the tradeoff manageable.

### Decomposing into services

I ended up splitting the backend into four services:

- **Gateway** — HTTP/WebSocket entry point for clients
- **Chat Service** — owns sessions and membership
- **Message Storage** — owns message persistence
- **Message Transmit** — stateless orchestrator that coordinates the other two during send

The decomposition I got wrong initially was where to put the "send a message" logic. My first instinct was to put it in Chat Service, since Chat Service already knew about sessions. That was wrong — it coupled two unrelated concerns (session metadata and message durability), and meant Chat Service would have ended up the biggest and most complicated service.

Pulling out Message Transmit as a pure orchestrator — no database, just coordination — made each of the other services much simpler. Transmit's entire job is: verify the sender with Chat Service, store the message with Message Storage, fetch the recipient list with Chat Service, return everything to Gateway.

**Lesson:** When a service grows faster than its neighbors, something is probably miscategorized. The shape of your services should track the shape of your data ownership, not the shape of your use cases.

### The send-message flow

By the end, the flow for sending a group-chat message looked like this:

1. Client POSTs `/api/messages/send` to Gateway.
2. Gateway validates the payload and calls Transmit via gRPC.
3. Transmit asks Chat Service: "is this user a member of this session?"
4. If yes, Transmit asks Message Storage to persist the message (atomic transaction on `messages` + `chat_sessions`).
5. Transmit asks Chat Service for the full member list.
6. Transmit returns the saved message and member list to Gateway.
7. Gateway looks up each recipient in Redis and pushes over WebSocket to whoever is online on this Gateway instance.

Tracing this flow end-to-end taught me something I hadn't internalized before: **every arrow in your architecture diagram is a potential failure point**. What happens if Chat Service is down during step 3? What if Message Storage times out during step 4? What if Gateway crashes between step 6 and step 7? Each of these has to have an answer, and the answers shape your error handling, your timeouts, and your transaction boundaries.

---

## 2. Deploying to AWS was harder than writing the code

Honestly, I didn't expect this. The code was maybe 60% of the work and felt like the hard part. Deployment turned out to be the other 40% and had more surprises per hour than coding ever did.

### What I had to learn

I came into this project knowing roughly what "the cloud" was. By the end I had a working mental model of:

- **ALB** — the load balancer that accepts public traffic and forwards to Gateway tasks
- **ECR** — where our built Docker images live
- **ECS / Fargate** — the container orchestrator that actually runs the tasks
- **Service Connect** — how ECS tasks find each other by logical name
- **IAM** — the permission model that controls what each task is allowed to do
- **Security Groups** — which tasks can talk to which other tasks on which ports
- **RDS / ElastiCache** — managed MySQL and Redis

Each of these has its own concepts, its own Terraform resource, and its own failure modes. I wrote ~10 separate Terraform modules to keep things organized — `network`, `security`, `ecr`, `logs`, `iam`, `alb`, `ecs_cluster`, `ecs_service`, `rds_mysql`, `elasticache_redis`. The `environments/dev/main.tf` then wires them all together.

### The MySQL initialization surprise

On my laptop, initializing MySQL was easy — the `docker-entrypoint-initdb.d` volume mount automatically runs the migration SQL files when the container starts. I assumed AWS would be similar.

It wasn't. RDS doesn't run init scripts. I had to create a **separate one-shot ECS task** whose only job was to connect to RDS and run the migrations. That was an uncomfortable realization at 10pm the night before a demo, because I thought everything was already set up — `terraform apply` had completed, the services were running, and then the API returned:

```
Table 'chatroom.chat_sessions' doesn't exist
```

**Lesson:** Local dev environments hide a lot of work that the cloud makes you do explicitly. Treating "it works on my laptop" as a signal that the cloud version will work is a mistake.

### Service discovery — the bug I'll remember forever

This one deserves its own section because it was the most frustrating and most educational issue I hit.

After `terraform apply` completed successfully, I tested the API:

```
$ curl -X POST http://<alb>/api/sessions/group ...
{"error":"failed to create group session"}
```

The CloudWatch logs showed:

```
CreateGroupChatSession error: rpc error: code = Unavailable
transport: Error while dialing:
lookup chatservice on 10.10.0.2:53: no such host
```

Gateway was trying to connect to the logical name `chatservice` — exactly what Service Connect is supposed to resolve — and the DNS lookup was failing. When I queried ECS directly:

```
aws ecs describe-services ...
  --query 'services[].serviceConnectConfiguration'
[ null, null, null, null ]
```

All four services reported their Service Connect config as `null`, even though Terraform said the apply had succeeded. The fix was to force-replace all four ECS services, which made them re-register from scratch:

```
terraform apply \
  -replace='module.chatservice_service.aws_ecs_service.this' \
  -replace='module.messagestorage_service.aws_ecs_service.this' \
  ...
```

After that, everything worked.

**What I think happened.** Service Connect needs two things to line up: the Cloud Map namespace has to be fully propagated, and each ECS task has to successfully register with the Envoy sidecar at startup. When Terraform created everything in one pass, there was a race — tasks started before the namespace was fully ready, silently failed to register, and from that point on Terraform didn't see any "change" that would trigger a restart. Forcing fresh tasks against a now-stable namespace bypassed the race.

**Course concept:** this is a textbook example of **eventual consistency** in a control plane. AWS's API returned success before the underlying state was fully propagated. Terraform trusted the API. The system looked healthy but wasn't.

**Lesson:** "`terraform apply` succeeded" is not the same as "the system is working." I'll be building a post-deploy verification step into all future deployments — something as simple as a health-check script that actually calls the API and confirms it returns 200 before the CI pipeline goes green.

---

## 3. What I'd do differently next time

### Make the database layer more efficient

The biggest thing is that my Chat Service and Message Storage talk to MySQL in a way that's correct but not optimized. For example, `BatchGetSessionSnapshots` in Message Storage loops through sessions one at a time and issues N queries — a classic N+1 query problem. For a dev environment with 10 sessions this is invisible, but it would dominate latency at scale.

Next time I'd:
- Batch these queries into a single `SELECT ... WHERE session_id IN (...)` with a `JOIN` to pull `last_seq` in one round trip.
- Add a connection pool size that actually matches the workload (right now the defaults are fine but I never tuned them).
- Use a read replica for the heavy read paths — message history, session lists — while keeping writes on the primary. This is a direct application of the *CQRS* pattern from the course.

The motivating goal: raise the **successful send rate per second** under load. This is exactly the kind of thing the experiment phase is for, and honestly I'd run the experiment earlier next time, so I had time to act on what I learned.

### Add the direct-session API

The Gateway only exposes `/api/sessions/group` to create group sessions. It never got a `/api/sessions/direct` endpoint for one-on-one chats. The database schema already supports it — `chat_sessions.type` can be `'direct'` — but the API and the service logic never got finished.

I realized partway through the project that direct sessions are genuinely harder than group sessions, in a few subtle ways:

- **Deduplication.** If user A and user B already have a direct session, a second "start a direct chat" call should return the *existing* session, not create a new one. This means the API is not a pure "create" — it's a "find-or-create". Enforcing this at the database level ideally needs an auxiliary table with a unique index on the sorted `(user_id_small, user_id_large)` pair, because MySQL can't naturally express "this session has exactly these two members" as a unique constraint on the existing tables.
- **Naming.** Group sessions have a name (`"demo-group"`). Direct sessions don't — the UI should show the name of *the other person* from the perspective of whoever is looking. That means the `name` field is effectively unused for direct sessions, which is an irregularity I wasn't sure how I wanted to handle.
- **Equivalence under argument order.** `CreateDirectSession(A, B)` and `CreateDirectSession(B, A)` should produce the same session. This is trivial in principle but requires disciplined normalization at the service boundary.

None of these are impossible — they just add a layer of business logic that group sessions don't need. In hindsight I should have built direct sessions first *because* they're harder, not skipped them because they were harder. The group path would have been a simpler specialization of the direct path.

**Lesson:** When two features share infrastructure but have different constraints, tackle the more constrained one first. It forces you to design for the harder case, and the easier case becomes free.

### Build verification into the deploy process

As mentioned above, I will never again trust a green `terraform apply` to mean the system is healthy. The next time, I'll write a small post-apply script that:
- Calls `/health` on Gateway.
- Makes one real write (create a session) and one real read.
- Returns non-zero if anything fails.

This turns "did the deploy work?" from a guess into a yes/no question.

---

## 4. Course concepts that turned out to matter most

Looking back, a handful of the course's conceptual building blocks really did map to specific decisions in this project:

- **Atomicity and isolation (ACID).** The monotonic `session_seq` only works because MySQL gives me a real transaction. A NoSQL store without transactions would have forced me to invent a consensus mechanism for sequence allocation, or accept out-of-order messages.
- **Service discovery.** Learning how docker-compose's internal DNS works locally, then learning how AWS Service Connect + Envoy does it in the cloud, made the abstraction click. The application code doesn't care which one is in play — it just connects to `chatservice:50051` — and that portability is the whole point.
- **Eventual consistency.** The Service Connect bug was eventual consistency biting me from inside AWS's control plane. The "single-instance push limitation" in Gateway is eventual consistency biting me from inside my own design: messages are in MySQL immediately but WebSocket delivery to the other Gateway instance is not. Both are the same idea.
- **Separation of concerns.** Splitting `internal/` from `cmd/main.go`, and splitting Transmit out from Chat Service, are both applications of the same principle — things that change for different reasons should live in different places.

---

## 5. The personal takeaway

The single biggest thing I'll carry away from this project is a healthier skepticism of what "done" means. Code that compiles is not code that runs. Code that runs locally is not code that runs in the cloud. An API that works once is not an API that works at scale. A `terraform apply` that succeeds is not a system that's healthy.

Building a distributed system is mostly the practice of not letting any of those illusions trick you into moving on too early. I didn't always avoid that trap in this project, but I'll avoid it more often in the next one.
