# GraphJin Components Extraction

This directory contains the extracted and isolated components from GraphJin's codebase for:

1. **Database Schema Introspection** - Automatic discovery of database tables, columns, relationships, and functions
2. **Auto-Join Algorithm** - Automatic relationship path finding between tables using a weighted directed graph

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Schema Introspection                         │
├─────────────────────────────────────────────────────────────────────┤
│  SQL Queries (PostgreSQL/MySQL)                                     │
│       │                                                             │
│       ▼                                                             │
│  DiscoverColumns() ─────► DBColumn structs                          │
│  DiscoverFunctions() ──► DBFunction structs                         │
│       │                                                             │
│       ▼                                                             │
│  NewDBInfo() ──────────► DBInfo (tables, columns, functions map)    │
│       │                                                             │
│       ▼                                                             │
│  NewDBSchema() ────────► DBSchema (relationship graph built)        │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                        Auto-Join Algorithm                          │
├─────────────────────────────────────────────────────────────────────┤
│  DBSchema.relationshipGraph (util.Graph)                            │
│       │                                                             │
│       ▼                                                             │
│  FindPath(from, to, through) ───► []TPath                           │
│       │                                                             │
│       │  Uses weighted shortest-path algorithm                      │
│       │  with min-heap for efficient path selection                 │
│       ▼                                                             │
│  PathToRel(TPath) ──────────────► DBRel (relationship info)         │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Component 1: Database Schema Introspection

### Location: `./schema/` and `./sql/`

### How It Works

#### Step 1: Database Discovery via SQL

GraphJin uses embedded SQL queries to introspect the database schema:

**PostgreSQL** (`sql/postgres_columns.sql`):
- Queries `pg_attribute`, `pg_class`, `pg_namespace`, `pg_constraint`
- Extracts: schema, table, column, type, constraints (PK, FK, unique), array status, full-text search capability
- Detects foreign key relationships automatically via `pg_constraint.confrelid`

**MySQL** (`sql/mysql_columns.sql`):
- Queries `information_schema.columns`, `information_schema.key_column_usage`, `information_schema.table_constraints`
- Uses UNION to merge column info with constraint info (workaround for MySQL limitations)

#### Step 2: Column Discovery (`tables.go`)

```go
func DiscoverColumns(db *sql.DB, dbtype string, blockList []string) ([]DBColumn, error)
```

Returns a slice of `DBColumn`:
```go
type DBColumn struct {
    ID          int32
    Name        string
    Type        string
    Array       bool
    NotNull     bool
    PrimaryKey  bool
    UniqueKey   bool
    FullText    bool
    FKRecursive bool      // Self-referential FK (e.g., parent_id -> id)
    FKeySchema  string    // Foreign key target schema
    FKeyTable   string    // Foreign key target table
    FKeyCol     string    // Foreign key target column
    // ...
}
```

#### Step 3: Function Discovery (`tables.go`)

```go
func DiscoverFunctions(db *sql.DB, dbtype string, blockList []string) ([]DBFunction, error)
```

Discovers stored functions/procedures with their input/output parameters.

#### Step 4: Building DBInfo (`tables.go`)

```go
func NewDBInfo(dbType, dbVersion, dbSchema, dbName string, cols []DBColumn, funcs []DBFunction, blockList []string) *DBInfo
```

Groups columns by table and creates a `DBTable` for each unique table. Tables from functions returning records are also added.

#### Step 5: Building DBSchema with Relationships (`schema.go`)

```go
func NewDBSchema(info *DBInfo, aliases map[string][]string) (*DBSchema, error)
```

This is where the **relationship graph** is built:

1. **Add nodes**: Each table becomes a node in the graph
2. **Add aliases**: Table aliases map to the same node
3. **Add relationships**: For each table:
   - `addColumnRels()`: Scans columns for foreign keys, determines relationship type
   - `addJsonRel()`: Handles embedded JSON relationships
   - `addPolymorphicRel()`: Handles polymorphic associations
   - `addRemoteRel()`: Handles remote API relationships

### Relationship Types (`schema.go`)

```go
const (
    RelNone RelType = iota
    RelOneToOne      // FK column has unique constraint
    RelOneToMany     // Standard FK relationship
    RelPolymorphic   // Type column + ID column pattern
    RelRecursive     // Self-referential (e.g., parent_id)
    RelEmbedded      // JSON/JSONB embedded data
    RelRemote        // External API
    RelSkip          // Skip this relationship
)
```

### Key Data Structures

```go
type DBSchema struct {
    tables            []DBTable               // All tables
    virtualTables     map[string]VirtualTable // Polymorphic tables
    dbFunctions       map[string]DBFunction   // DB functions
    tindex            map[string]nodeInfo     // schema:table -> node ID
    edgesIndex        map[string][]edgeInfo   // Relationship lookup
    allEdges          map[int32]TEdge         // All graph edges
    relationshipGraph *util.Graph             // The actual graph
}
```

---

## Component 2: Auto-Join Algorithm

### Location: `./autojoin/dwg.go` and `./util/graph.go`

### How It Works

The auto-join algorithm finds the optimal path between two tables through the relationship graph.

#### The Graph Structure (`util/graph.go`)

```go
type Graph struct {
    edgeID int32                    // Auto-incrementing edge ID
    edges  map[[2]int32][]Edge      // [from,to] -> edges
    graph  [][]int32                // Adjacency list
}

type Edge struct {
    ID     int32
    OppID  int32   // Opposite edge ID (bidirectional)
    Weight int32   // Cost/priority
    Name   string  // Column name that created this edge
}
```

#### Building the Graph (`autojoin/dwg.go`)

When a foreign key relationship is discovered:

```go
func (s *DBSchema) addToGraph(lti DBTable, lcol DBColumn, rti DBTable, rcol DBColumn, rt RelType) error
```

1. **Two edges are created** (bidirectional):
   - `table -> foreign_key_table` (forward)
   - `foreign_key_table -> table` (reverse)

2. **Edges are indexed** by:
   - Table name: `edgesIndex["users"] = [...]`
   - Relationship name: `edgesIndex["author"] = [...]` (derived from `author_id`)

3. **Edge weights** determine priority:
   - Regular FK: weight = 1
   - Embedded JSON: weight = 5
   - Remote: weight = 8
   - Recursive: weight = 10
   - Polymorphic: weight = 15

#### Path Finding Algorithm (`autojoin/dwg.go`)

```go
func (s *DBSchema) FindPath(from, to, through string) ([]TPath, error)
```

**Algorithm Steps:**

1. **Lookup edge indices** for `from` and `to` tables
2. **Call `between()`** to find paths between all possible node combinations
3. **Call `pickPath()`** which uses `AllPaths()` from the graph

**AllPaths Algorithm** (`util/graph.go`):

Uses a **modified Dijkstra's algorithm** with a min-heap:

```go
func (g *Graph) AllPaths(from, to int32) [][]int32 {
    h := newHeap()
    h.push(path{weight: 0, parent: from, nodes: []int32{from}})
    visited := make(map[[2]int32]struct{})
    
    for len(*h.paths) > 0 {
        p := h.pop()  // Get minimum weight path
        // Check if reached destination
        // Explore neighbors, accumulate paths
        // Limit to 3000 iterations for safety
    }
    return paths
}
```

**Edge Selection** (`pickEdges()`):

Given a path of nodes `[A, B, C, D]`:
1. First edge: Must match the `from` edge info
2. Last edge: Must match the `to` edge info  
3. Middle edges: Pick minimum weighted edge (avoiding opposite edges to prevent loops)

#### Result: TPath

```go
type TPath struct {
    Rel RelType    // Relationship type
    LT  DBTable    // Left table
    LC  DBColumn   // Left column
    RT  DBTable    // Right table
    RC  DBColumn   // Right column
}
```

### Example: Auto-Join in Action

Given this schema:
```
users (id, name)
posts (id, user_id → users.id, title)
comments (id, post_id → posts.id, user_id → users.id, body)
```

Query: `FindPath("comments", "users", "")`

The algorithm:
1. Finds two possible paths:
   - `comments.user_id → users.id` (direct, weight 1)
   - `comments.post_id → posts.id, posts.user_id → users.id` (2 hops, weight 2)
2. Returns the shortest path (direct relationship)

---

## API Modification Points for Your DSL

### 1. Schema Discovery Customization

**Current API:**
```go
di, err := GetDBInfo(db, "postgres", blockList)
schema, err := NewDBSchema(di, aliases)
```

**Potential Modifications:**
- Add custom relationship discovery hooks
- Support additional databases (SQLite, CockroachDB)
- Allow manual relationship overrides
- Add schema versioning/caching

### 2. Path Finding Customization

**Current API:**
```go
path, err := schema.FindPath(from, to, through)
rel := PathToRel(path[0])
```

**Potential Modifications:**
- Expose path weighting configuration
- Add path cost constraints (max hops)
- Support "avoid these tables" constraints
- Return multiple paths ranked by cost
- Add relationship metadata to results

### 3. Relationship Type Extension

**Current Types:**
```go
RelOneToOne, RelOneToMany, RelPolymorphic, RelRecursive, RelEmbedded, RelRemote
```

**Potential Additions:**
- `RelManyToMany` (join table detection)
- `RelComposite` (multi-column foreign keys)
- `RelTemporal` (time-based relationships)
- Custom DSL relationship types

### 4. Query Generation Hooks

The path result feeds into query generation (`qcode.go`):
```go
sel.Rel = sdata.PathToRel(path[0])
sel.Joins = append(sel.Joins, Join{Rel: rel, Filter: buildFilter(rel, pid)})
```

**Potential Modifications:**
- Custom JOIN type selection (INNER, LEFT, FULL)
- Join condition customization
- Aggregate relationship handling
- Subquery generation options

---

## File Reference

| File | Purpose |
|------|---------|
| `schema/tables.go` | DB discovery, DBInfo/DBTable construction |
| `schema/schema.go` | DBSchema, relationship building |
| `schema/sql.go` | Embedded SQL query references |
| `schema/funcs.go` | Standard aggregate functions |
| `schema/strings.go` | String representations |
| `autojoin/dwg.go` | Graph edges, path finding |
| `util/graph.go` | Core graph data structure |
| `util/heap.go` | Min-heap for path finding |
| `sql/*.sql` | Database introspection queries |

---

## Dependencies

- `database/sql` - Standard Go database interface
- `golang.org/x/sync/errgroup` - Parallel query execution
- No external graph libraries (custom implementation)

---

## Usage Example

```go
package main

import (
    "database/sql"
    _ "github.com/lib/pq"
    
    "your-project/extracted/schema"
)

func main() {
    db, _ := sql.Open("postgres", "...")
    
    // 1. Discover schema
    dbInfo, _ := schema.GetDBInfo(db, "postgres", nil)
    
    // 2. Build relationship graph
    dbSchema, _ := schema.NewDBSchema(dbInfo, nil)
    
    // 3. Find path between tables
    path, _ := dbSchema.FindPath("comments", "users", "")
    
    // 4. Use relationship info
    rel := schema.PathToRel(path[0])
    // rel.Type, rel.Left.Col, rel.Right.Col, etc.
}
```
