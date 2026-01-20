# Included Files

This document lists all files included in the GraphJin Schema Introspection & Auto-Join Algorithm extraction.

## Directory Structure

```
graphjin-components-extraction/
├── README.md                              # Main documentation
├── FILES.md                               # This file
└── extracted/                             # All extracted components
    ├── go.mod                            # Go module definition
    ├── go.sum                            # Go dependencies checksum
    ├── temp_test.go                      # Usage examples and tests
    │
    ├── schema/                           # Database schema introspection
    │   ├── SCHEMA_INTROSPECTION.md       # Schema introspection documentation
    │   ├── funcs.go                      # Standard SQL aggregate functions
    │   ├── schema.go                     # DBSchema core - relationship graph
    │   ├── sql.go                        # Embedded SQL query loader
    │   ├── strings.go                    # String formatting utilities
    │   ├── tables.go                     # Column/function discovery, DBInfo
    │   ├── test_dbinfo.go                # Test data helpers
    │   │
    │   └── sql/                          # SQL introspection queries
    │       ├── mysql_columns.sql         # MySQL column discovery query
    │       ├── mysql_functions.sql       # MySQL function discovery query
    │       ├── mysql_info.sql            # MySQL database info query
    │       ├── postgres_columns.sql      # PostgreSQL column discovery query
    │       ├── postgres_functions.sql    # PostgreSQL function discovery query
    │       └── postgres_info.sql         # PostgreSQL database info query
    │
    ├── autojoin/                         # Auto-join algorithm
    │   ├── AUTOJOIN_ALGORITHM.md         # Auto-join algorithm documentation
    │   └── dwg.go                        # Directed weighted graph operations
    │
    └── util/                             # Graph utilities
        ├── graph.go                      # Core graph data structure
        └── heap.go                       # Min-heap for pathfinding
```

## Core Components

### 1. Schema Introspection (`extracted/schema/`)

**Main Files:**
- `tables.go` - Core discovery functions
  - `GetDBInfo()` - Main entry point for schema discovery
  - `DiscoverColumns()` - Column and relationship discovery
  - `DiscoverFunctions()` - Stored function discovery
  - `NewDBInfo()` - DBInfo constructor

- `schema.go` - Relationship graph construction
  - `NewDBSchema()` - Build relationship graph from DBInfo
  - `FindPath()` - Find join path between tables
  - `GetFirstDegree()` - Get direct relationships
  - `GetSecondDegree()` - Get 2-hop relationships
  - `addColumnRels()` - Add FK relationships to graph
  - `addJsonRel()` - Add JSON embedded relationships
  - `addPolymorphicRel()` - Add polymorphic relationships

- `sql.go` - SQL query embedding
  - `getSQLStmt()` - Load appropriate SQL for database type

- `funcs.go` - Standard SQL functions
  - `GetFunctions()` - Return list of built-in aggregate functions

- `strings.go` - Utilities
  - String representation methods for debugging

**SQL Query Files (`extracted/schema/sql/`):**
- PostgreSQL queries:
  - `postgres_columns.sql` - Introspect columns via pg_catalog
  - `postgres_functions.sql` - Introspect functions via pg_proc
  - `postgres_info.sql` - Get database version and schema

- MySQL queries:
  - `mysql_columns.sql` - Introspect columns via information_schema
  - `mysql_functions.sql` - Introspect routines via information_schema
  - `mysql_info.sql` - Get database version and schema

**Key Types:**
```go
type DBInfo struct {
    Type      string        // "postgres" or "mysql"
    Version   int
    Schema    string
    Name      string
    Tables    []DBTable
    Functions []DBFunction
}

type DBTable struct {
    Schema      string
    Name        string
    Type        string      // "table", "view", "function"
    Columns     []DBColumn
}

type DBColumn struct {
    Name        string
    Type        string
    FKeySchema  string    // Foreign key target
    FKeyTable   string
    FKeyCol     string
    PrimaryKey  bool
    UniqueKey   bool
}

type DBSchema struct {
    tables            []DBTable
    relationshipGraph *util.Graph
    edgesIndex        map[string][]edgeInfo
}
```

### 2. Auto-Join Algorithm (`extracted/autojoin/`)

**Main Files:**
- `dwg.go` - Directed weighted graph for table relationships
  - Graph construction from foreign keys
  - Path finding algorithm
  - Edge weight calculation
  - Relationship type detection

**Key Functions:**
```go
func (s *DBSchema) FindPath(from, to, through string) ([]TPath, error)
func (s *DBSchema) addToGraph(lti DBTable, lcol DBColumn, rti DBTable, rcol DBColumn, rt RelType) error
func (s *DBSchema) pickPath(from, to edgeInfo, through string) (graphResult, error)
```

**Key Types:**
```go
type TPath struct {
    Rel RelType    // Relationship type
    LT  DBTable    // Left table
    LC  DBColumn   // Left column
    RT  DBTable    // Right table
    RC  DBColumn   // Right column
}

type TEdge struct {
    From   int32
    To     int32
    Weight int32
    Type   RelType
    LT     DBTable
    LC     DBColumn
    RT     DBTable
    RC     DBColumn
}
```

### 3. Graph Utilities (`extracted/util/`)

**Main Files:**
- `graph.go` - Core graph implementation
  - `NewGraph()` - Create new directed graph
  - `AddNode()` - Add table node
  - `AddEdge()` - Add relationship edge
  - `AllPaths()` - Find all paths using modified Dijkstra
  - `GetEdges()` - Get edges between nodes
  - `Connections()` - Get connected nodes

- `heap.go` - Min-heap for pathfinding
  - Priority queue for Dijkstra's algorithm
  - Path weight ordering

**Key Types:**
```go
type Graph struct {
    edgeID int32
    edges  map[[2]int32][]Edge
    graph  [][]int32    // Adjacency list
}

type Edge struct {
    ID     int32
    OppID  int32   // Opposite edge (bidirectional)
    Weight int32   // Cost/priority
    Name   string
}
```

## Dependencies

From `go.mod`:
```
require (
    github.com/lib/pq v1.10.9           // PostgreSQL driver
    golang.org/x/sync v0.6.0            // Concurrent utilities (errgroup)
)
```

## Testing & Examples

- `extracted/temp_test.go` - Basic usage example showing:
  1. Schema discovery
  2. Relationship graph construction
  3. Path finding between tables

## Documentation Files

- `README.md` - Main documentation with architecture, usage, and examples
- `extracted/schema/SCHEMA_INTROSPECTION.md` - Detailed schema introspection docs
- `extracted/autojoin/AUTOJOIN_ALGORITHM.md` - Detailed auto-join algorithm docs
- `FILES.md` - This file

## Total File Count

- **Go source files**: 10
- **SQL query files**: 6
- **Documentation files**: 4
- **Module files**: 2 (go.mod, go.sum)
- **Total**: 22 files

## Lines of Code (Approximate)

- Schema introspection: ~1,500 LOC
- Auto-join algorithm: ~450 LOC
- Graph utilities: ~300 LOC
- SQL queries: ~400 LOC
- Tests/examples: ~100 LOC
- **Total**: ~2,750 LOC

## Removed

All full GraphJin source code has been removed. Only the two core components remain:
1. Database schema introspection
2. Auto-join algorithm with graph utilities

This represents approximately 5% of the original GraphJin codebase, focused solely on these two independent capabilities.