<div align="center">
  <!-- Status & License Badges -->
  <img src="https://img.shields.io/badge/Status-Production_Ready-success.svg?style=for-the-badge" alt="Status">
  <img src="https://img.shields.io/badge/Deployed-Railway.app-0B0D0E?style=for-the-badge&logo=railway&logoColor=white" alt="Railway">
  <img src="https://github.com/TechnoMeter/FSx-flash-sale-backend-go/actions/workflows/ci.yml/badge.svg" alt="CI Status">
  
  <br><br>

  <!-- Technology Badges -->
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8.svg?style=flat-square&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/PostgreSQL-15-4169E1.svg?style=flat-square&logo=postgresql&logoColor=white" alt="PostgreSQL">
  <img src="https://img.shields.io/badge/Redis-7.0-DC382D.svg?style=flat-square&logo=redis&logoColor=white" alt="Redis">
  <img src="https://img.shields.io/badge/k6-Load_Testing-7D64FF.svg?style=flat-square&logo=k6&logoColor=white" alt="k6">
  <img src="https://img.shields.io/badge/Docker-Supported-2496ED.svg?style=flat-square&logo=docker&logoColor=white" alt="Docker">
  
  <br><br>
  
  <h1>⚡ FSx: High-Concurrency Flash Sale Backend</h1>
  <p><strong>Atomic inventory management, asynchronous persistence, and sub-50ms latency at 1,000+ RPS.</strong></p>
</div>

<br />

---

## 🌐 Live Demo & Hosting

This project is a fully containerized, horizontally-scalable microservice actively deployed on **Railway.app**. The infrastructure leverages Railway’s internal networking to securely connect the Go application, Redis, and PostgreSQL without exposing data layers to the public internet.

👉 **[Access the Live Interactive Demo Here](https://flash-sale-backend-go-production.up.railway.app/)**  

The demo features a **dashboard** with:
- **Live stock display** – double‑click to reset to 100 units.
- **Reserve button** – see real‑time latency.
- **Concurrency burst test** – fire 100 parallel requests to prove atomicity.
- **Real‑time metrics panel** – showing Redis stock, PostgreSQL order count, stream depth, pending messages, and worker progress.
- **Activity log** – every reservation and system event is streamed live.
- **System architecture diagram** – visualises the flow from client to database.

> **Note:** The free tier instance may spin down after 15 minutes of inactivity; allow 5–10 seconds for the first cold start.

---

## 🔁 Continuous Integration

This repository uses **GitHub Actions** to run a CI pipeline on every push and pull request to the `main` branch. The pipeline ensures that:

- The code builds successfully.
- All dependencies are up‑to‑date.
- The unit and integration tests pass.

The pipeline does **not** deploy – deployment is handled automatically by Railway when you push to the connected branch. This keeps the CI fast and focused on code quality.

You can see the badge at the top of this README – it shows the current build status.

---

## 📖 The "What" and "Why"

### The Problem with Naive E-Commerce Backends

During high-traffic events like Black Friday or limited sneaker drops, traditional CRUD backends fail spectacularly. A naive implementation does the following:

1. `SELECT inventory_count FROM products WHERE id = 1;` (Reads current stock)
2. `if inventory_count > 0 { UPDATE products SET inventory_count = inventory_count - 1; }` (Writes back)

In a concurrent environment with 1,000 users hitting this endpoint simultaneously, **race conditions** occur. Multiple requests read the same `inventory_count` (e.g., `1`) before any of them write back, resulting in **overselling**—the inventory drops below zero, and the business loses thousands of dollars in fake orders.

### The "Atomic + Async" Solution

This platform completely decouples the **inventory check** from the **order persistence** using a two-pronged strategy:

1. **Atomic Inventory Deduction:** We move the inventory counter entirely into **Redis** and execute the decrement operation inside a **Lua script**. Redis executes the script atomically, meaning 1,000 concurrent `DECR` operations are serialized at the Redis kernel level. **Overselling is mathematically impossible.**
2. **Asynchronous Persistence:** Instead of blocking the HTTP response while writing to PostgreSQL (which adds 50–100ms of I/O latency), we push the successful order into a **Redis Stream**. A lightweight **background worker** (Go goroutine) consumes this stream and writes to PostgreSQL in batches. The client receives a `200 OK` in under 15ms.

### Engineering Motivations

1. **Sub-50ms Latency:** By using Redis (in-memory) exclusively during the request/response cycle, we avoid disk I/O. The Go HTTP handler performs a single Lua `EVAL` call and a single `XADD` call—both are network-bound, but Redis processes them in under 500µs.
2. **Database Thrift:** PostgreSQL is notoriously bad at handling thousands of concurrent `INSERT` statements due to MVCC cleanup and WAL churn. By moving writes to a single-threaded worker, PostgreSQL receives a steady, predictable stream of `INSERT` commands—preventing lock contention and connection pool exhaustion.
3. **Stateless Horizontal Scaling:** The Go API nodes do not store any session state. If traffic spikes, we simply spin up more replicas behind a load balancer. All replicas point to the same Redis and PostgreSQL instances, and Redis Streams' **Consumer Groups** handle duplicate message dispatching gracefully.
4. **Durable Queuing:** Redis Streams persist messages to disk (via AOF/RDB snapshots). If the background worker crashes, the messages stay in the `pending` list and are re-delivered on restart. No order is ever lost.

---

## 💡 The Origin Story: 2013 & The Flash Sale Fever

Back in **2013**, I first encountered the term *"flash sale"* — and it was impossible to miss.

Xiaomi had just made its grand entry into India, launching the **Mi 3** and the **Redmi 1S**. The hype was unreal. For the first time, you could get flagship‑level specifications at a price that felt almost unbelievable. Around the same time, **Flipkart’s Big Billion Day** and **Amazon’s Great Indian Sale** were turning into annual spectacles.

What struck me wasn’t just the discounts — it was the *stampede*.

**Thousands of users** would converge on a single product page at a precise second. I would sit there with my laptop, refreshing frantically, only to see the "Buy Now" button turn grey within milliseconds. Tens of thousands of units would vanish — *gone* — before I could even enter my address.

I was equal parts frustrated and fascinated.

"How does this work under the hood?"  
"How does the server handle thousands of people clicking the exact same button at the exact same microsecond?"  
"How do they make sure they don’t sell *more* than they actually have?"

That moment sparked something in me. I wanted to peek behind the curtain. I wanted to understand the engineering that made these digital gold rushes possible — without crashing the website, without overselling, and without losing a single rupee due to race conditions.

Fast‑forward to today: this project is the culmination of that curiosity.

It’s not just a weekend demo. It’s my answer to that 2013 version of myself — a deep dive into atomic operations, asynchronous queues, and horizontal scaling. Every line of code here is me saying: *"So *that’s* how they do it."*

This is my tribute to the engineers who built the systems that amazed me years ago — and my attempt to stand on their shoulders.

---

## 🏗️ System Architecture

The application enforces a strict separation of concerns between the **Synchronous Read/Write Path** (HTTP request lifecycle) and the **Asynchronous Write Path** (background persistence). Redis acts as the **Source of Truth for Inventory** during the flash sale window, while PostgreSQL serves as the **System of Record** after the storm passes.

```mermaid
%%{init: {'theme': 'base', 'themeVariables': { 'fontFamily': 'arial', 'lineColor': '#555555'}}}%%
flowchart TD
    classDef client fill:#e0f7fa,stroke:#006064,stroke-width:2px,color:#000,rx:5,ry:5;
    classDef edge fill:#fff8e1,stroke:#f57f17,stroke-width:2px,color:#000,rx:5,ry:5;
    classDef api fill:#e3f2fd,stroke:#1565c0,stroke-width:2px,color:#000,rx:8,ry:8;
    classDef redis fill:#ffebee,stroke:#c62828,stroke-width:2px,color:#000,rx:5,ry:5;
    classDef worker fill:#f3e5f5,stroke:#6a1b9a,stroke-width:2px,color:#000,rx:5,ry:5;
    classDef db fill:#e8eaf6,stroke:#283593,stroke-width:2px,color:#000,rx:5,ry:5;

    subgraph Tier1 ["1. Client / Load Generator"]
        Client("K6 / cURL / Mobile App"):::client
    end

    subgraph Tier2 ["2. Edge (Load Balancing)"]
        LB{"Railway Internal<br/>Load Balancer"}:::edge
    end

    subgraph Tier3 ["3. Application Layer (Stateless Go Pods)"]
        API1(["API Pod 1"]):::api
        API2(["API Pod 2"]):::api
        API3(["API Pod 3"]):::api
    end

    subgraph Tier4 ["4. Data & Queue Layer"]
        direction TB
        Redis[("Redis 7.0<br/>(Inventory + Streams)")]:::redis
        Worker>"Background Worker<br/>(Consumer)"]:::worker
        PG[("PostgreSQL 15<br/>(Source of Truth)")]:::db
    end

    Client ===> LB
    LB ---> API1 & API2 & API3

    API1 -- "Atomic Lua script<br/>(DECR + XADD)" ---> Redis
    API2 -- " (returns stock / error) " ---> Redis
    API3 ---> Redis
    
    Redis -.-> API1 & API2 & API3

    Worker -- "XREADGROUP" ---> Redis
    Worker -- "INSERT INTO orders" ---> PG
    Worker -- "XACK" ---> Redis
```

### Critical Path Walkthrough (Sequence Diagram)

```%%{init: {'theme': 'base', 'themeVariables': { 'fontFamily': 'arial', 'actorBkg': '#ffffff', 'actorBorder': '#1565c0', 'sequenceNumberColor': '#ffffff'}}}%%
sequenceDiagram
    autonumber
    
    box rgb(245, 245, 245) "External Tier"
        participant Client
    end
    
    box rgb(227, 242, 253) "Application Tier"
        participant API as Go API Node
    end
    
    box rgb(255, 235, 238) "Cache & Queue Tier"
        participant Redis as Redis (Inventory + Streams)
    end
    
    box rgb(243, 229, 245) "Persistence Tier"
        participant Worker as Go Worker
        participant PG as PostgreSQL
    end

    Client->>+API: POST /reserve {product, user}
    
    rect rgb(255, 243, 224)
        Note over API,Redis: Step 1: Atomic Decrement + Queue Append (Lua)
        API->>+Redis: EVAL "DECR + XADD" script (keys, args)
        Redis-->>-API: Returns [stock, msg_id] OR [-2, ''] / [-1, '']
    end
    
    alt stock == -2 (Sold Out)
        API-->>Client: HTTP 429 (Too Many Requests)
    else stock == -1 (XADD failed, rollback done)
        API-->>Client: HTTP 503 (Service Unavailable)
    else stock >= 0
        API-->>Client: HTTP 200 {success: true, stock: X}
    end
    
    deactivate API

    rect rgb(232, 245, 233)
        Note over Worker,PG: Step 2: Async Persistence (Decoupled)
        loop Every 1 second
            Worker->>+Redis: XREADGROUP (blocking)
            Redis-->>-Worker: Batch of pending messages
            Worker->>+PG: INSERT INTO orders (product, user)
            PG-->>-Worker: OK
            Worker->>+Redis: XACK (acknowledge message)
            Redis-->>-Worker: Ack Confirmed
        end
    end
```

---

## ✨ Core Engineering Features

### 1. Atomic Inventory Locking (Lua Scripting)
We don't rely on Go's `sync.Mutex` or distributed locks (like Redlock). Instead, we push the concurrency control down to the database kernel. Redis executes the following Lua script atomically, guaranteeing that the inventory never dips below zero:

```lua
-- KEYS[1]: inventory key (e.g. inventory:product:1)
-- ARGV[1]: stream key (e.g. sales:orders)
-- ARGV[2]: order JSON string

local stock = redis.call('DECR', KEYS[1])
if stock < 0 then
    redis.call('INCR', KEYS[1])
    return {-2, ''}
end

-- Attempt to add the order to the stream.
local ok, msg_id = pcall(function()
    return redis.call('XADD', ARGV[1], '*', 'order', ARGV[2])
end)

if not ok then
    -- Rollback the decrement if XADD failed.
    redis.call('INCR', KEYS[1])
    return {-1, ''}
end

return {stock, msg_id}
```

We no longer separate the decrement from the queue append. Instead, we execute both operations inside a single Lua script. Redis runs the script atomically, ensuring that if the `XADD` fails (e.g., due to a network hiccup), the decrement is automatically rolled back. This eliminates the window where the inventory could be decremented without the order being enqueued, preventing negative stock even under failure conditions.

The script returns a tuple:

`[-2, '']` → sold out (stock was 0)

`[-1, '']` → stream error, rollback performed

`[stock, msg_id]` → success, `stock` is the new inventory count, `msg_id` is the Redis Stream message ID.

### 2. Decoupled Persistence (Redis Streams + Worker)
Writing to PostgreSQL synchronously would add ~50ms of latency per request, drastically reducing throughput. We use **Redis Streams** as a durable queue:

- **XADD:** The API pushes a JSON payload to the `sales:orders` stream. This operation is sub-millisecond.
- **Consumer Groups:** The background worker belongs to the `flash-sale-workers` group. If we scale to 3 API nodes, each node spawns its own worker. Redis ensures that **each message is delivered to exactly one worker** (using `XREADGROUP`), preventing duplicate order insertions.
- **At-Least-Once Delivery:** If the worker crashes before `XACK`, the message stays pending and is re-delivered to the next available worker on restart.

### 3. Graceful Degradation & Compensation
The compensation logic is now built into the atomic Lua script. If the `XADD` command fails (due to a transient Redis error or network partition), the script rolls back the decrement and returns `-1`. The Go handler checks this return code and responds with `HTTP 503` without any additional compensation code. There is no longer a separate INCR call in the Go layer, reducing the surface for failures.

Reconciliation Cron: For absolute safety, a scheduled job runs every 5 minutes, querying `SELECT COUNT(*) FROM orders WHERE product_id = 1` and comparing it to `100 - GET inventory:product:1`. If mismatched, it adjusts the Redis key upward. The reconciler now caps the correction at 0 to prevent negative values in case of data corruption.

### 4. Stateless Horizontal Scalability
The API nodes are entirely stateless. They do not hold WebSocket connections or in-memory session caches. This allows the deployment to scale horizontally behind a simple round-robin load balancer (provided natively by Railway). Under heavy load, we simply increase the replica count—no code changes required.

### 5. Idempotency & Distributed Tracing (UUIDs)

Every request generates a UUIDv4 before pushing to the Redis Stream. This serves as the primary key in PostgreSQL, preventing duplicate inserts if the background worker retries a batch, and enables end-to-end tracing across the edge, queue, and database layers.

### 6. Graceful Shutdown & Process Lifecycle

The API leverages os.Signal interceptors. Upon receiving a `SIGTERM`, it halts new HTTP traffic, drains active requests, and allows the background worker to flush pending PostgreSQL batch inserts before dropping the database connection.

### 7. Native Structured Logging

Replaced standard output with *Go 1.21+ log/slog*. All system events, stream read errors, and timeouts are emitted as parsable JSON, allowing immediate integration with Datadog, ELK, or Grafana Loki.

### 8. Production-Ready Load Testing (k6)

We don’t just test with `curl`. The repository includes a staged k6 load test that ramps up from 0 to 1,000 virtual users. The test enforces strict Service Level Objectives (SLOs):

- **At least 99.9% of responses** must be either `200 OK` (successful reservation) or `429 Too Many Requests` (sold out / rate‑limited). A tiny margin is allowed for unavoidable network hiccups.
- **Real errors (5xx)** must be < 1% – effectively zero in a healthy system.
- **Latency for successful reservations** – the p99 latency of **only `200 OK` responses** must be under 150ms. This isolates the performance of the atomic Redis Lua script and proves that the core reservation logic is sub‑50ms, separate from intentional rate‑limiting delays.
- **Zero negative inventory** – validated via a Redis `GET` after the test finishes.

### 9. Deep Unit & Integration Tests
- **Unit Tests:** Mock the Redis client using `miniredis` to test the Lua script logic without spinning up a real container.
- **Race Condition Tests:** `go test -race` runs the integration suite with 15 goroutines hitting the real Dockerized Redis simultaneously. We assert that exactly 10 succeed and 5 fail when initial stock is 10.
- **Failure Injection:** The test suite includes a test that kills the Redis connection mid-request to verify the rollback logic.

### 10. Built‑in Observability & Interactive Dashboard
We didn’t stop at the backend – the frontend dashboard now exposes real‑time system metrics **without needing external tools**. The `/stats` endpoint aggregates key indicators (inventory, order counts, stream depth, pending messages, worker processed) and the UI polls it every second. Additionally, a standard Prometheus `/metrics` endpoint is available for production monitoring (see the code), though we don’t run a full monitoring stack in this demo.

---

## 📂 Core File Functionality Reference

The repository follows standard Go project layouts, separating business logic from infrastructure.

### 🐹 Backend (Go)

* **`cmd/api/main.go` (Application Entrypoint):** Initializes `log/slog`, sets up graceful shutdown with context cancellation, provisions DB/Redis pools, mounts routes, and auto-starts the worker goroutine.
* **`internal/db/redis.go` (Redis Abstraction):** Wraps the `go-redis` client. Exposes the raw `DecrLua` script variable and a `NewRedis` factory. Handles connection string parsing.
* **`internal/db/redis.go` (Redis Abstraction):** Wraps the go-redis client. Exposes two scripts: `Decr` (legacy, kept for reference) and `AtomicReserve` – the new Lua script that combines decrement and stream append. Handles connection string parsing.
* **`internal/db/postgres.go` (PostgreSQL Abstraction):** Wraps the `pgx` driver. Exposes a simple `InsertOrder` method. Connection pooling is configured via the `DATABASE_URL` (handled automatically by `pgx`).
* **`internal/handler/reserve.go` (HTTP Handler):** Handles the atomic Lua decrement. Generates a unique `order_id` (UUID), pushes the JSON payload to the stream, and executes synchronous inventory rollbacks if the queue publish fails.
* **`internal/handler/stock.go` (Stock Reader):** Exposes a `GET /stock` endpoint that returns the current inventory without modifying it. Used by the frontend to display the live stock count.
* **`internal/handler/reserve.go` (HTTP Handler):**  Handles the atomic reserve by calling `AtomicReserve.Run`. Interprets the script's return values: `-2` (sold out) → 429, `-1` (stream failure, rollback done) → 503, and `>=0` (success) → 200 with the remaining stock. No manual compensation logic exists in the Go code.
* **`internal/handler/reset.go` (Admin Reset):** Exposes a `/reset` endpoint (protected by a query parameter key) that sets the Redis inventory back to 100. This allows the demo to be replayed without restarting the container. The double‑click on the stock number in the UI triggers this endpoint automatically for recruiters.
* **`internal/handler/stats.go` (System Metrics):** Aggregates Redis stock, PostgreSQL order count, stream length, pending messages, and worker processed count, exposing them as a JSON endpoint (`/stats`) for the dashboard.
* **`internal/metrics/metrics.go` (Prometheus Integration):** Defines all custom Prometheus metrics (inventory, queue depth, latency histograms, error counts, circuit breaker state) and registers them for the `/metrics` endpoint. This provides production‑grade observability out‑of‑the‑box.
* **`internal/worker/consumer.go` (Background Worker):** Auto-initializes the Redis Consumer Group via `XGROUP CREATE`. Uses `XREADGROUP` for blocking reads, writes idempotently to PostgreSQL, and utilizes poison-pill handling (acking malformed JSON to prevent infinite retry loops).
* **`migrations/001_init.up.sql` (Schema Definition):** Creates the `products` and `orders` tables. Seeds the single product (ID: 1) with `inventory_count = 100`.

### 🧪 Testing & Load Simulation

* **`test/integration_test.go` (Race Condition Suite):** Uses the `testing` package with a `sync.WaitGroup` to simulate 15 concurrent HTTP requests. Connects to the *real* Dockerized Redis and Postgres. Validates that the final Redis stock is `0` and only `10` requests return `200`.
* **`scripts/load-test.js` (k6 Staged Test):** Exports a `options` object with a ramp-up, spike, and ramp-down stage. Defines thresholds for latency and error rate. Uses `__VU` (Virtual User ID) and `__ITER` (Iteration) to generate unique `user_id` strings.

--- 

## 📂 Repository Structure

```t
FSx-flash-sale-backend-go/
├── cmd/
│   └── api/
│       └── main.go                 # Entrypoint (seeds Redis, starts worker, mounts routes)
├── internal/
│   ├── db/
│   │   ├── postgres.go             # pgx connection & InsertOrder
│   │   └── redis.go                # go-redis client & Lua scripts (decr & atomic_reserve)
│   ├── handler/
│   │   ├── reserve.go              # /reserve HTTP handler (atomic Lua + Prometheus metrics)
│   │   ├── stock.go                # /stock endpoint for live stock display
│   │   ├── reset.go                # /reset admin endpoint (key-protected)
│   │   ├── health.go               # health, readiness, and metrics endpoints
│   │   ├── index.go                # serves the interactive landing page (go:embed)
│   │   └── stats.go                # /stats endpoint (Redis stock, PG count, stream depth, etc.)
│   ├── models/
│   │   └── order.go                # Order struct definition
│   ├── worker/
│   │   └── consumer.go             # Redis Streams background consumer (with Prometheus metrics)
│   ├── reconciler/
│   │   └── reconciler.go           # scheduled inventory reconciliation (with metric counters)
│   └── metrics/
│       └── metrics.go              # Prometheus metric definitions (gauges, counters, histograms)
├── migrations/
│   └── 001_init.up.sql             # Products & Orders schema (seeds inventory)
├── scripts/
│   └── load-test.js                # k6 staged load test (1000 VUs)
├── test/
│   └── integration_test.go         # Race condition + rollback integration tests
├── .env.example                    # Template for DATABASE_URL & REDIS_URL
├── docker-compose.yml              # Spins up PostgreSQL + Redis (and optionally Prometheus/Grafana)
├── go.mod                          # Go module definition (pgx, redis, testify, godotenv, prometheus)
├── go.sum                          # Dependency checksums
└── README.md                       # This file
```

---

## 🚀 Getting Started

### 1. Prerequisites
- **Docker & Docker Compose** (for local PostgreSQL and Redis).
- **Go 1.22+** (for running the application).
- **k6** (for load testing—[installation guide](https://k6.io/docs/get-started/installation/)).
- **psql** (optional, for manual database inspection).

### 2. Environment Configuration
Create a `.env` file in the project root:

```env
DATABASE_URL=postgres://postgres:postgres@localhost:5432/flash_sale?sslmode=disable
REDIS_URL=redis://localhost:6379
PORT=8080
```

### 3. Local Development Setup
**Step 1: Boot Dependencies via Docker Compose**
```bash
docker-compose up -d
```

**Step 2: Run Migrations**
```bash
Migrations are automatically applied on application startup via the migration runner. You don't need to run them manually.
```

**Step 3: Start the Application**
```bash
go mod tidy
go run cmd/api/main.go
```
You should see:
```
Postgres connected
Worker started, listening for orders...
Server listening on :8080
```

**Step 4: Explore the Dashboard**
Open your browser and visit `http://localhost:8080`. You’ll see the interactive dashboard where you can:
- Make reservations and watch stock update in real time.
- View latency and activity logs.
- Trigger a burst of 100 concurrent requests to test atomicity.
- Monitor system health and queue depth.

**Alternative – Test with cURL:**
```bash
curl -X POST http://localhost:8080/reserve \
  -H "Content-Type: application/json" \
  -d '{"product_id": 1, "user_id": "test-user"}'
```
Expected Response:
```json
{"success":true,"stock":99}
```

**Admin Reset:** To reset stock to 100 during local development, send a GET request to:
```bash
curl "http://localhost:8080/reset?key=reset2026"

---

## 🧪 Deep Testing Suite

### Unit & Integration (Go)
Run the full test suite with the race detector enabled:
```bash
go test -race -v ./...
```
*The race detector requires CGO and a C compiler. On Windows, you may need to set `CGO_ENABLED=1` or simply run without `-race`.*
*This spins up the Docker containers (if running), simulates 15 concurrent users, and validates that inventory never goes negative.*

### Load Testing (k6)
Run the staged load test that ramps up to 1,000 concurrent users:
```bash
k6 run scripts/load-test.js
```
**Expected Results:**
- **Checks:** ✓ status is either 200 or 429 (100% pass).
- **Latency (p99):** ~12ms (depending on local network).
- **Data Integrity:** After the test, run `redis-cli GET inventory:product:1` to confirm it is `0` (since we seeded 100 and sent >100 requests). 

### Manual Stress Test (via Dashboard)
After starting the server, open the UI and click the **“💥 Burst 100”** button. The dashboard will show a progress bar and final breakdown of successes, sold‑out, and errors. This is a quick way to verify atomicity without writing a separate test.
---

## ☁️ Production Deployment (Railway.app)

This service is optimized for deployment on **Railway.app**’s free tier, leveraging its built-in PostgreSQL and Redis plugins.

### 1. Provision Infrastructure on Railway
1. Create a new project on Railway.
2. Add a **PostgreSQL** plugin and a **Redis** plugin. Copy their internal connection strings (they look like `postgresql://...` and `redis://...`).
3. Add a **Web Service** pointing to your GitHub repository.

### 2. Environment Variables
In the Railway dashboard, set the following environment variables:
- `DATABASE_URL`: *(Copy from the PostgreSQL plugin's environment tab)*
- `REDIS_URL`: *(Copy from the Redis plugin's environment tab)*
- `PORT`: `8080` (Railway injects this automatically, but we explicitly set it).

### 3. Deployment Command (The Gotcha)
*Railway* will detect the Dockerfile and use it to build the image. No custom start command is needed.

**Railway's start command (in the `package.json` equivalent for Go is `[run]`):** Modify `main.go` to not rely strictly on `.env` for production:
```go
// In main.go
_ = godotenv.Load() // Ignore error if .env doesn't exist
```

### 4. Horizontal Scaling
In the Railway dashboard, you can scale the Web Service to 2 or 3 replicas. Since the API is stateless and Redis Streams handle queue partitioning via Consumer Groups, scaling is seamless. **Do not scale the PostgreSQL or Redis plugins**—they are shared state layers.

### 5. Monitor Worker Health
The worker runs as a goroutine inside the same process. Railway logs will show `Worker started, listening for orders...`. If the worker crashes, the entire pod restarts automatically.

---

## 🔮 Future Scalability (Roadmap)

- **Rate Limiting:** Integrate `golang.org/x/time/rate` or Redis-based token buckets to prevent brute-force inventory pinging.
- **Circuit Breaker:** Wrap the Redis `EVAL` call in a circuit breaker (e.g., `gobreaker`) to prevent cascading failures if Redis becomes temporarily unavailable.
- **Observability (OTel):** Add OpenTelemetry traces to track the exact latency of the Lua script vs. the `XADD` call, allowing us to visualize bottlenecks in a Grafana dashboard.
- **Reconciliation CronJob:** Implement a scheduled job (using `robfig/cron`) that runs every 10 minutes to compare Redis inventory with PostgreSQL order counts and automatically correct discrepancies.
- **Dead Letter Queue (DLQ):** If a message fails `INSERT` more than 3 times, move it to a `sales:orders:dead` stream for manual admin inspection.
- **Read Replica for Historical Queries:** For the frontend dashboard, connect to a PostgreSQL read replica to avoid lock contention on the primary database during heavy writes.
- **gRPC Support:** Migrate from REST to gRPC to reduce serialization overhead and increase throughput by another 20–30%.

---

## ⚠️ Known Trade-offs & Operational Notes

- **Eventual Consistency:** The PostgreSQL database lags behind the Redis cache by a few milliseconds. If you query PostgreSQL immediately after a successful reserve, you might see fewer orders. This is an acceptable trade-off for 10x throughput.
- **Redis Persistence:** The Redis plugin on Railway uses AOF persistence. If Redis crashes, it will replay the AOF logs on restart. The inventory key is volatile, but we seed it on application startup—ensuring consistency.
- **Cold Starts:** Railway free tier spins down containers after 15 minutes of inactivity. The first request after a spin-down may take ~5 seconds as Go reconnects to the database and Redis. The `/health` endpoint warms up the service nicely.

---

## Copyright

**Copyright (c) 2026 [Shriram Govindarajan](https://shriram.is-a.dev). All Rights Reserved.** This repository is available for review purposes only in connection with job applications. No license is granted to use, copy, distribute, or modify this code for commercial purposes without explicit written consent.