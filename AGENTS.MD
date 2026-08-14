# Finance Monitor Backend — Agent Guide

This document is the source of truth for coding agents working on the Finance Monitor Go backend.

Read this file before making architectural or implementation changes.

---

# 1. Project Goal

Finance Monitor is a personal finance monitoring application.

The backend is being migrated from Google Apps Script + Google Sheets to:

```text
Client
├── MacroDroid
├── PWA
└── Flutter
      │
      ▼
Go REST API
      │
      ▼
PostgreSQL
```

Google Sheets must no longer be treated as the primary database.

Eventually Google Sheets may remain only for:

- export
- reporting
- backup
- manual inspection

The Go backend must become the source of truth.

---

# 2. Technology

Backend:

```text
Go 1.26+
net/http
pgx/v5
PostgreSQL
```

Current Go module:

```text
finance-monitor/backend
```

Do not introduce a web framework unless there is a strong technical reason.

Prefer Go standard library.

Do not introduce ORM.

Database access uses:

```text
github.com/jackc/pgx/v5
github.com/jackc/pgx/v5/pgxpool
```

---

# 3. Architecture

Use feature-oriented architecture.

Current structure should follow approximately:

```text
backend/
├── cmd/
│   └── api/
│       └── main.go
│
├── internal/
│   ├── account/
│   ├── category/
│   ├── config/
│   ├── database/
│   ├── notification/
│   ├── parser/
│   ├── server/
│   └── transaction/
│
├── migrations/
├── go.mod
└── go.sum
```

For application features use:

```text
HTTP Handler
      ↓
Service
      ↓
Repository
      ↓
PostgreSQL
```

Responsibilities:

## Handler

Responsible for:

- HTTP request decoding
- HTTP status codes
- HTTP response encoding

Handler must NOT contain SQL.

Handler must NOT contain significant business rules.

## Service

Responsible for:

- business rules
- validation
- orchestration

Service must NOT know HTTP concepts.

## Repository

Responsible for:

- PostgreSQL access
- SQL queries
- persistence

Repository must NOT contain HTTP logic.

---

# 4. Dependency Injection

Use constructor-based dependency injection.

Example:

```go
repository := account.NewRepository(db)

service := account.NewService(repository)

handler := account.NewHandler(service)
```

Do not use mutable global database variables.

Do not introduce dependency injection frameworks.

---

# 5. Money Representation

Never use floating point for transaction amounts.

Use:

```go
int64
```

Example:

```text
Rp100.000
```

is stored as:

```text
100000
```

Database:

```sql
BIGINT
```

Do not use:

```text
FLOAT
DOUBLE
REAL
```

for monetary amounts.

Confidence values are allowed to use decimal/float because they are not money.

---

# 6. Core Transaction Model

The new backend uses exactly three transaction types:

```text
income
expense
transfer
```

Do NOT reintroduce the legacy types:

```text
TRANSFER_IN
TRANSFER_OUT
```

A transfer must be represented as ONE transaction.

Example:

```text
SeaBank → ShopeePay Rp100.000
```

becomes:

```text
type                   transfer
amount                 100000
source_account_id      SeaBank
destination_account_id ShopeePay
```

Do not create synthetic counterpart transactions.

This intentionally differs from the old GAS implementation.

---

# 7. Transaction Rules

## Income

Required:

```text
type = income
destination_account_id != NULL
category type = income
```

Normally:

```text
source_account_id = NULL
```

Example:

```text
Salary → Bank Jago
```

---

## Expense

Required:

```text
type = expense
source_account_id != NULL
category type = expense
```

Normally:

```text
destination_account_id = NULL
```

Example:

```text
SeaBank → Food
```

---

## Transfer

Required:

```text
type = transfer
source_account_id != NULL
destination_account_id != NULL
category_id = NULL
```

And:

```text
source_account_id != destination_account_id
```

---

# 8. Account Balance

Do NOT treat a mutable balance column as the financial source of truth.

Account current balance is calculated from:

```text
opening_balance
+ income
+ incoming transfers
- expenses
- outgoing transfers
```

Conceptually:

```text
current_balance =
opening_balance

+ SUM(income where destination = account)

+ SUM(transfer where destination = account)

- SUM(expense where source = account)

- SUM(transfer where source = account)
```

Transaction history is the source of truth.

---

# 9. Existing Database Tables

Current schema includes:

```text
accounts
categories
transactions
raw_notifications
parser_rules
category_rules
```

Do not manually modify production schema outside migrations.

Every schema change must have a new migration.

Migration naming:

```text
001_initial_schema.sql
002_parser_support.sql
003_example.sql
```

Never edit an already-applied migration to represent a new schema change.

Create a new migration instead.

---

# 10. Raw Notification Pipeline

Notifications from MacroDroid must enter:

```text
POST /api/v1/notifications
```

Pipeline:

```text
MacroDroid
    ↓
HTTP endpoint
    ↓
validation
    ↓
fingerprint
    ↓
raw_notifications
    ↓
parser
    ↓
transaction
```

Raw notifications must be preserved even if parsing fails.

Possible raw notification states:

```text
pending
parsed
ignored
failed
```

---

# 11. Notification Deduplication

Raw notifications use a SHA-256 fingerprint.

Fingerprint is derived from notification data such as:

```text
source_app
title
body
received_at
```

PostgreSQL has a UNIQUE index on fingerprint.

Database uniqueness is the final protection against concurrency/race conditions.

Do not rely only on:

```text
SELECT first → INSERT later
```

for duplicate prevention.

---

# 12. Parser Architecture

Parser package must be pure domain logic.

Expected package:

```text
internal/parser/
├── model.go
├── parser.go
├── normalize.go
├── money.go
├── promo.go
├── account.go
├── seabank.go
├── shopeepay.go
├── generic.go
└── *_test.go
```

Parser code must NOT:

- access PostgreSQL
- access HTTP
- read environment variables
- write transactions
- mutate application state

Parser receives input and returns parsing results.

---

# 13. Parser Interface

Preferred model:

```go
type Input struct {
    SourceApp string
    Title     string
    Text      string
    BigText   string
}
```

Parser result:

```go
type Result struct {
    Type                   string
    Amount                 int64

    SourceAccountName      string
    DestinationAccountName string

    Merchant               string
    CategoryName           string
    Description            string

    ParseStatus            string
    Confidence             float64

    Ignore                 bool
}
```

Parser interface:

```go
type Parser interface {
    CanParse(input Input) bool
    Parse(input Input) (*Result, error)
}
```

---

# 14. Parser Priority

Preserve this conceptual ordering from the legacy GAS behavior:

```text
QRIS detection
      ↓
promotional notification filter
      ↓
dynamic parser rules
      ↓
application-specific parser
      ↓
generic parser
```

QRIS must be evaluated before promotional filtering.

This order exists intentionally to avoid valid QRIS notifications being incorrectly classified.

Dynamic database rules are not implemented yet.

Until implemented:

```text
QRIS
↓
promo filter
↓
specific parser
↓
generic parser
```

---

# 15. Supported Account Mapping

Current legacy mapping:

```text
seabank
→ SeaBank

shopeepay
→ ShopeePay

jago
→ Bank Jago

livin
mandiri
→ Mandiri

brimo
bri
→ BRI

flip
→ Flip

tokopedia
→ Tokopedia

shopee
→ Shopee
```

Owned account detection currently focuses on:

```text
SeaBank
ShopeePay
Bank Jago
Mandiri
BRI
```

Avoid assuming every application is an owned financial account.

Example:

```text
Tokopedia
Shopee
```

may be notification sources without being balance-holding accounts.

---

# 16. Legacy GAS Parser Behavior

Legacy GAS currently handles:

## SeaBank

- QRIS payments
- incoming funds
- payment success
- ShopeePay top ups

## ShopeePay

- balance top-up success

## Generic parser

- incoming transfer-like notifications
- outgoing transfer-like notifications
- expenses
- income
- unknown transactions

Legacy transaction types:

```text
INCOME
EXPENSE
TRANSFER_IN
TRANSFER_OUT
```

must be adapted to the normalized Go model.

---

# 17. QRIS

QRIS is considered:

```text
expense
```

Example:

```text
Pembayaran QRIS untuk WARUNG MAKMUR sebesar Rp25.000
```

Expected approximate result:

```text
Type              expense
Amount            25000
SourceAccountName SeaBank
Merchant          WARUNG MAKMUR
CategoryName      Belum Dikategorikan
ParseStatus       AUTO
Confidence        0.99
```

If merchant extraction fails:

```text
Confidence = 0.90
```

---

# 18. Promotion Filtering

Known transaction indicators include:

```text
berhasil
telah masuk
menerima dana
dana masuk
pembayaran qris
pembayaran sebesar
transfer sebesar
top up berhasil
isi saldo berhasil
```

Known promotional indicators include:

```text
promo
diskon
bonus cashback
lebih hemat
dapatkan tiket
voucher
penawaran
buruan
khusus untukmu
khusus untuk kamu
```

Transaction indicators take precedence over promotional wording.

Example:

```text
Pembayaran berhasil dan dapat cashback
```

must NOT automatically be ignored as promotion.

---

# 19. Dynamic Parser Rules

Legacy GAS has a dynamic rule engine.

Target PostgreSQL table:

```text
parser_rules
```

Rules may:

```text
PARSE
IGNORE
```

Potential rule matching inputs:

```text
source_app
keyword
```

Rule output may contain:

```text
transaction_type
category
merchant
confidence
```

Rules also have:

```text
is_active
priority
```

When implemented, lower priority value should generally execute first unless otherwise documented.

Do not implement this until explicitly requested.

---

# 20. Category Rules

Legacy GAS has category rules based on keyword matching against:

```text
merchant
description
account
```

Target table:

```text
category_rules
```

Rules point to:

```text
category_id
```

and include:

```text
confidence
is_active
priority
```

Only expenses should normally pass through spending categorization.

Transfer should not become a spending category.

Income uses income categories.

---

# 21. Parse Metadata

Transactions may store:

```text
merchant
parse_status
confidence
source
```

Typical parse statuses:

```text
AUTO
RULE
MANUAL
NEEDS_REVIEW
REPROCESS
```

Typical source values:

```text
manual
notification
reprocess
import
```

Do not expose arbitrary internal error information to API clients.

---

# 22. PostgreSQL Rules

Use parameterized SQL.

Correct:

```go
db.QueryRow(
    ctx,
    `SELECT ... WHERE id = $1`,
    id,
)
```

Never build SQL using request strings:

```go
"SELECT ... " + input
```

Always propagate:

```go
context.Context
```

through:

```text
Handler
↓
Service
↓
Repository
↓
PostgreSQL
```

---

# 23. HTTP API

Current API namespace:

```text
/api/v1
```

Known routes include:

```text
GET  /
GET  /api/v1/health

GET  /api/v1/accounts
POST /api/v1/accounts

GET  /api/v1/categories
POST /api/v1/categories

GET  /api/v1/transactions
POST /api/v1/transactions

POST /api/v1/notifications
```

Do not create database-oriented endpoints such as:

```text
/getSheetRows
/insertDatabaseRow
```

Design endpoints around domain resources.

---

# 24. JSON Convention

Success responses currently follow approximately:

```json
{
  "success": true,
  "data": {}
}
```

Errors:

```json
{
  "success": false,
  "error": "message"
}
```

Maintain consistency unless a deliberate API response refactor is performed across the application.

Do not leak PostgreSQL errors to clients.

---

# 25. Validation

Business rules must exist server-side.

Never rely on Flutter/PWA validation as the only validation.

Clients may help users avoid invalid input, but backend remains authoritative.

---

# 26. Error Handling

Do not panic for normal bad input.

Prefer sentinel/domain errors when useful.

Examples:

```go
ErrInvalidAmount
ErrInvalidType
ErrCategoryRequired
```

Translate domain errors to HTTP in handler layer.

Unexpected errors:

```text
HTTP 500
```

Client receives generic message.

Detailed error goes to server logs.

---

# 27. Coding Style

Always:

```bash
gofmt
go test ./...
```

before finishing a task.

Prefer:

- small functions
- explicit errors
- simple constructors
- standard library
- table-driven tests

Avoid:

- unnecessary abstractions
- generic repository frameworks
- reflection
- ORM
- global mutable state
- premature microservices
- giant service files

The project is a personal finance application, not an enterprise distributed system.

Keep architecture clean but practical.

---

# 28. Testing Requirements

Parser functionality must have unit tests.

Use table-driven tests.

Financial business logic should have tests where practical.

Important test categories:

```text
money parsing
text normalization
promotion filtering
QRIS
SeaBank income
SeaBank expense
SeaBank transfer
ShopeePay top up
generic parser
transaction validation
duplicate notification behavior
```

No parser change should silently break previously supported notification formats.

When fixing a parsing bug:

1. add failing test reproducing the notification
2. fix parser
3. verify all tests

---

# 29. Deployment Target

Current goal:

```text
Rp0/month
```

Planned architecture:

```text
PWA / Flutter / MacroDroid
          │
          ▼
Go API
Render free tier
          │
          ▼
PostgreSQL
Neon free tier
```

Deployment provider must not affect domain architecture.

Backend accesses database only through:

```text
DATABASE_URL
```

Local:

```text
PostgreSQL local
```

Production:

```text
Neon PostgreSQL
```

Application code should not care which PostgreSQL provider is being used.

---

# 30. Environment Variables

Expected variables will include at least:

```text
PORT
DATABASE_URL
```

Future production configuration will include an ingestion API secret.

Never commit secrets.

Never hardcode:

```text
database passwords
API keys
tokens
credentials
```

---

# 31. Security TODO

Before production cutover, implement authentication for notification ingestion.

MacroDroid must not be allowed to POST freely to a public unauthenticated endpoint.

Expected minimum protection:

```http
Authorization: Bearer <secret>
```

Secret must come from environment configuration.

Do not implement unless explicitly requested by the current task.

---

# 32. Migration from Google Sheets

Do not delete legacy GAS/Sheets until migration is verified.

Migration strategy:

```text
GAS + Sheets
     │
     │ temporary coexistence
     ▼
Go + PostgreSQL
```

Old data will later be imported:

```text
Google Sheets
      ↓
migration/import tool
      ↓
PostgreSQL
```

Data parity must be checked before GAS is retired.

---

# 33. Current Progress

Completed:

```text
[x] Go module/bootstrap
[x] HTTP server
[x] health endpoint
[x] PostgreSQL local connection
[x] pgx connection pool
[x] initial migrations
[x] accounts
[x] account calculated balance
[x] categories
[x] transactions
[x] transaction validation
[x] income/expense/transfer model
[x] raw notification ingestion
[x] SHA-256 notification fingerprint
[x] database-level notification deduplication
[x] parser support schema
```

Currently being implemented:

```text
[ ] parser package
[ ] SeaBank parser
[ ] ShopeePay parser
[ ] QRIS parser
[ ] generic parser
[ ] parser unit tests
```

Pending:

```text
[ ] integrate parser with notification service
[ ] resolve parser account names to account IDs
[ ] resolve parser category names to category IDs
[ ] transaction creation from notifications
[ ] raw status parsed/ignored/failed
[ ] dynamic parser_rules engine
[ ] category_rules engine
[ ] reprocess raw notification API
[ ] ingestion API authentication
[ ] CORS configuration if required
[ ] graceful shutdown
[ ] production migration execution strategy
[ ] Neon production database
[ ] Render deployment
[ ] health/readiness improvements
[ ] import existing Google Sheets data
[ ] MacroDroid endpoint cutover
[ ] verify parity against GAS
[ ] retire GAS as primary backend
```

---

# 34. Approximate Milestones

Current overall progress toward GAS replacement and production deployment:

```text
~55–60%
```

Expected:

```text
~70%
parser package complete

~80%
notification → parser → transaction pipeline complete

~90%
production security/config/migrations complete

~95%
Render + Neon deployed

~100%
legacy data migrated
MacroDroid switched
GAS no longer primary backend
```

These percentages are approximate engineering milestones, not strict task counts.

---

# 35. Agent Rules

Before editing:

1. inspect existing implementation
2. inspect relevant migration
3. inspect tests
4. preserve existing behavior
5. follow this document

When requested to implement one phase:

- stay within that phase
- do not implement unrelated future features
- do not rewrite existing architecture
- do not rename public API without instruction
- do not alter database semantics without migration
- do not delete working code merely to simplify the task

If implementation requires violating this document:

STOP.

Explain:

```text
what conflicts
why it conflicts
recommended change
affected files
```

Do not silently change architecture.

---

# 36. Task Completion Format

After every coding task, report:

## Files Created

```text
...
```

## Files Modified

```text
...
```

## Implementation

Short explanation.

## Behavior Changes

List intentional behavior changes.

If none:

```text
None.
```

## Tests

Include exact result of:

```bash
go test ./...
```

## Remaining TODO

Only TODO directly related to the completed phase.

Do not automatically start the next phase.
