# GraphJin Schema Introspection & Auto-Join Algorithm

This repository contains the extracted and isolated core components from GraphJin:

1. **Database Schema Introspection** - Automatic discovery of database tables, columns, relationships, and functions
2. **Auto-Join Algorithm** - Automatic relationship path finding between tables using a weighted directed graph

These components have been extracted from the full GraphJin codebase to provide a standalone library for database schema discovery and intelligent join path finding.

---

## 📁 Project Structure

```
extracted/
├── schema/              # Database schema introspection
│   ├── sql/            # SQL queries for PostgreSQL and MySQL
│   ├── tables.go       # Column and function discovery
│   ├── schema.go       # Relationship graph construction
│   ├── funcs.go        # Standard SQL functions
│   ├── sql.go          # Embedded SQL queries
│   └── strings.go      # String utilities
├── autojoin/           # Auto-join algorithm
│   └── dwg.go         # Path finding and relationship detection
├── util/              # Graph utilities
│   ├── graph.go       # Directed weighted graph implementation
│   └── heap.go        # Min-heap for pathfinding
├── go.mod
├── go.sum
└── temp_test.go       # Usage examples
```

---

## 🏗️ Architecture Overview

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
│  NewDBInfo() ──────────► DBInfo (tables, columns, functions)        │
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

## 🚀 Quick Start

### Installation

```bash
go get github.com/yourusername/graphjin-extracted
```

### Basic Usage

```go
package main

import (
    "database/sql"
    "fmt"
    _ "github.com/lib/pq"
    
    "github.com/yourusername/graphjin-extracted/schema"
)

func main() {
    // Connect to database
    db, err := sql.Open("postgres", "postgres://user:pass@localhost/mydb?sslmode=disable")
    if err != nil {
        panic(err)
    }
    defer db.Close()
    
    // 1. Discover database schema
    dbInfo, err := schema.GetDBInfo(db, "postgres", nil)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Discovered %d tables\n", len(dbInfo.Tables))
    
    // 2. Build relationship graph
    dbSchema, err := schema.NewDBSchema(dbInfo, nil)
    if err != nil {
        panic(err)
    }
    
    // 3. Find join path between tables
    paths, err := dbSchema.FindPath("comments", "users", "")
    if err != nil {
        panic(err)
    }
    
    // 4. Use the relationship information
    if len(paths) > 0 {
        rel := schema.PathToRel(paths[0])
        fmt.Printf("Join: %s.%s -> %s.%s\n", 
            rel.Left.Table, rel.Left.Col,
            rel.Right.Table, rel.Right.Col)
    }
}
```

---

## 📖 Component Details

### 1. Database Schema Introspection

#### Supported Databases
- PostgreSQL
- MySQL

#### Discovery Process

**Step 1: Column Discovery**
```go
columns, err := schema.DiscoverColumns(db, "postgres", blockList)
```

Discovers:
- Table and column names
- Data types
- Constraints (PK, FK, UNIQUE, NOT NULL)
- Foreign key relationships
- Array types
- Full-text search capability

**Step 2: Function Discovery**
```go
functions, err := schema.DiscoverFunctions(db, "postgres", blockList)
```

Discovers stored procedures and functions with their parameters.

**Step 3: Build Schema**
```go
dbInfo := schema.NewDBInfo(dbType, version, schema, name, columns, functions, blockList)
dbSchema, err := schema.NewDBSchema(dbInfo, aliases)
```

Creates a complete schema with:
- All tables and columns indexed
- Relationship graph constructed
- Foreign key relationships mapped
- Virtual tables for polymorphic associations

#### Relationship Types

```go
const (
    RelNone         // No relationship
    RelOneToOne     // FK with unique constraint
    RelOneToMany    // Standard FK
    RelPolymorphic  // Type column + ID pattern
    RelRecursive    // Self-referential
    RelEmbedded     // JSON/JSONB embedded
    RelRemote       // External API
    RelSkip         // Skip relationship
)
```

---

### 2. Auto-Join Algorithm

#### How It Works

The auto-join algorithm uses a **weighted directed graph** to find optimal join paths between tables.

**Graph Structure:**
- **Nodes**: Database tables
- **Edges**: Foreign key relationships (bidirectional)
- **Weights**: Relationship costs (lower = preferred)

**Edge Weights:**
- Regular FK: 1
- Embedded JSON: 5
- Remote relation: 8
- Recursive: 10
- Polymorphic: 15

#### Path Finding

```go
paths, err := dbSchema.FindPath(fromTable, toTable, throughTable)
```

**Algorithm:**
1. Uses modified Dijkstra's algorithm with min-heap
2. Finds all paths between tables
3. Ranks paths by total weight
4. Returns shortest/cheapest paths first

**Features:**
- Direct relationships preferred over multi-hop
- Avoids circular paths
- Optional "through" table constraint
- Handles multiple valid paths

#### Example: Complex Join

Given schema:
```
users (id, name)
posts (id, user_id → users.id, title)
comments (id, post_id → posts.id, user_id → users.id, body)
likes (id, comment_id → comments.id, user_id → users.id)
```

Query: Find path from `likes` to `users`

**Found paths:**
1. `likes.user_id → users.id` (direct, weight 1) ✓ Selected
2. `likes.comment_id → comments.id → users.id` (weight 2)
3. `likes.comment_id → comments.id → posts.id → users.id` (weight 3)

The algorithm returns path #1 as the optimal join.

---

## 🔧 API Reference

### Schema Package

#### Types

```go
type DBInfo struct {
    Type      string        // "postgres" or "mysql"
    Version   int           // Database version
    Schema    string        // Database schema name
    Name      string        // Database name
    Tables    []DBTable     // All discovered tables
    Functions []DBFunction  // All discovered functions
}

type DBTable struct {
    Schema      string
    Name        string
    Type        string      // "table", "view", "function"
    Columns     []DBColumn
    PrimaryCol  DBColumn
    SecondaryCol DBColumn
}

type DBColumn struct {
    Name        string
    Type        string
    Array       bool
    NotNull     bool
    PrimaryKey  bool
    UniqueKey   bool
    FKeySchema  string    // FK target schema
    FKeyTable   string    // FK target table
    FKeyCol     string    // FK target column
}

type DBSchema struct {
    // Internal - use provided methods
}
```

#### Functions

```go
// Discover database schema
func GetDBInfo(db *sql.DB, dbType string, blockList []string) (*DBInfo, error)

// Build relationship graph
func NewDBSchema(info *DBInfo, aliases map[string][]string) (*DBSchema, error)

// Find join path between tables
func (s *DBSchema) FindPath(from, to, through string) ([]TPath, error)

// Get all tables with direct relationships
func (s *DBSchema) GetFirstDegree(t DBTable) ([]RelNode, error)

// Get all tables within 2 hops
func (s *DBSchema) GetSecondDegree(t DBTable) ([]RelNode, error)

// Convert path to relationship
func PathToRel(path TPath) DBRel
```

---

## 🎯 Use Cases

### 1. Query Builder DSLs
Automatically generate JOIN clauses based on table names.

### 2. GraphQL to SQL
Build SQL queries from GraphQL schemas using relationship discovery.

### 3. Database Documentation
Generate ER diagrams and relationship documentation.

### 4. Migration Tools
Understand dependencies before altering schemas.

### 5. Data Lineage
Track data flow through table relationships.

### 6. ORM Optimization
Discover optimal join strategies for complex queries.

---

## 🧪 Testing

```bash
cd extracted
go test ./...
```

See `temp_test.go` for usage examples.

---

## 📝 Notes

### Blocklist

You can exclude tables/columns from discovery:

```go
blockList := []string{
    "internal_*",      // Exclude tables starting with "internal_"
    "*.secret_column", // Exclude specific column across all tables
}

dbInfo, err := schema.GetDBInfo(db, "postgres", blockList)
```

### Table Aliases

Support multiple names for the same table:

```go
aliases := map[string][]string{
    "users": {"authors", "editors"},
}

dbSchema, err := schema.NewDBSchema(dbInfo, aliases)

// Now you can use any of these names:
path, _ := dbSchema.FindPath("posts", "authors", "")
```

### Performance

- Schema discovery runs once at startup
- Graph pathfinding is O(E log V) where E=edges, V=vertices
- Results can be cached for repeated queries
- Typically < 1ms for path finding in graphs with hundreds of tables

---

## 🤝 Contributing

This is an extraction from the [GraphJin](https://github.com/dosco/graphjin) project. 

Original authors and contributors deserve credit for this excellent work.

---

## 📄 License

Apache License 2.0 (inherited from GraphJin)

---

## 🙏 Acknowledgments

Extracted from [GraphJin](https://github.com/dosco/graphjin) by Vikram Rangnekar and contributors.

These components represent the core intelligence of GraphJin's automatic join generation system.