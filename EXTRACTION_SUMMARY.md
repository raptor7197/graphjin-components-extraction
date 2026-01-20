
## Extracted Components

### 1. Database Schema Introspection

**Purpose**: Automatically discover database tables, columns, relationships, and functions by querying system catalogs.

**Files Extracted**:
- `extracted/schema/tables.go` - Core discovery logic
- `extracted/schema/schema.go` - Relationship graph construction
- `extracted/schema/sql.go` - SQL query loader
- `extracted/schema/funcs.go` - Standard SQL functions
- `extracted/schema/strings.go` - Utilities
- `extracted/schema/test_dbinfo.go` - Test helpers
- `extracted/schema/sql/postgres_columns.sql`
- `extracted/schema/sql/postgres_functions.sql`
- `extracted/schema/sql/postgres_info.sql`
- `extracted/schema/sql/mysql_columns.sql`
- `extracted/schema/sql/mysql_functions.sql`
- `extracted/schema/sql/mysql_info.sql`

**what they do **:
- Discover all tables and columns
- Detect foreign key relationships
- Identify primary keys, unique constraints
- Discover stored functions/procedures
- Support PostgreSQL and MySQL
- Handle blocklists for excluding tables/columns

### 2. the auto join algo 

**Purpose**: to Automatically find optimal join paths between database tables using a weighted directed graph.

**Files Extracted**:
- `extracted/autojoin/dwg.go` - Directed weighted graph operations
- `extracted/util/graph.go` - Core graph data structure
- `extracted/util/heap.go` - Min-heap for Dijkstra's algorithm

**Capabilities**:
- Build relationship graph from foreign keys
- Find shortest path between any two tables
- Support "through" table constraints
- Handle multiple relationship types (1:1, 1:M, polymorphic, recursive)
- Weight-based path optimization
- Bidirectional edge support

- HTTP server
- WebSocket support
- Authentication/authorization
- Rate limiting
- Caching layer
- API gateway features

### Dependencies Removed

The extraction removed most dependencies, keeping only:
- `database/sql` (standard library)
- `github.com/lib/pq` (PostgreSQL driver)
- `golang.org/x/sync` (errgroup for parallel queries)

## File Structure Comparison

### Before Extraction
```
graphjin/
├── core/            (GraphQL to SQL compiler)
├── serv/            (HTTP server)
├── auth/            (Authentication)
├── plugin/          (Plugin system)
├── cmd/             (CLI tools)
├── conf/            (Configuration)
├── benchmark/       (Benchmarks)
├── examples/        (Example apps)
├── tests/           (Integration tests)
├── website/         (Documentation site)
└── ... (500+ files)
```



### 3. Documentation Added
- `README.md` - Main documentation with architecture and examples
- `QUICKSTART.md` - Quick start guide
- `FILES.md` - Complete file reference
- `EXTRACTION_SUMMARY.md` - This document
- `extracted/schema/SCHEMA_INTROSPECTION.md` - Schema discovery details
- `extracted/autojoin/AUTOJOIN_ALGORITHM.md` - Algorithm details

### 4. Test Files
- Added `extracted/temp_test.go` with usage examples
- Removed full GraphJin test suites

## Use Cases for Extracted Components

These isolated components are useful for:

1. **Custom Query Builders**: Auto-generate JOIN clauses
2. **GraphQL Libraries**: Build custom GraphQL-to-SQL compilers
3. **ORM Development**: Intelligent relationship detection
4. **Schema Documentation**: Generate ER diagrams
5. **Migration Tools**: Analyze schema dependencies
6. **Database Diff Tools**: Compare schema versions
7. **API Generators**: Auto-generate REST/GraphQL APIs from schema
8. **Data Lineage**: Track relationships across tables
