# Database Schema Introspection Deep Dive

## Overview

The schema introspection system in GraphJin automatically discovers your database structure including tables, columns, relationships, functions, and constraints. This information is then used to:

1. Validate GraphQL queries against actual schema
2. Build the relationship graph for auto-joins
3. Generate GraphQL types for introspection
4. Enforce type safety at query time

## Introspection Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Database Server                              │
│                    (PostgreSQL / MySQL)                              │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    SQL Introspection Queries                         │
│  ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐    │
│  │ *_columns.sql    │ │ *_functions.sql  │ │ *_info.sql       │    │
│  │ - Table/Column   │ │ - Stored procs   │ │ - DB version     │    │
│  │ - Foreign keys   │ │ - Parameters     │ │ - Schema name    │    │
│  │ - Constraints    │ │ - Return types   │ │ - DB name        │    │
│  └──────────────────┘ └──────────────────┘ └──────────────────┘    │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      DiscoverColumns()                               │
│                      DiscoverFunctions()                             │
│  - Execute SQL queries                                               │
│  - Parse results into DBColumn and DBFunction structs                │
│  - Handle database-specific quirks                                   │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        NewDBInfo()                                   │
│  - Group columns by table                                            │
│  - Create DBTable structs                                            │
│  - Build lookup maps (colMap, tableMap)                              │
│  - Handle function-based tables                                      │
└───────────────────────────┬─────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       NewDBSchema()                                  │
│  - Add table nodes to graph                                          │
│  - Process aliases                                                   │
│  - Discover relationships from FKs                                   │
│  - Build edge index for fast lookup                                  │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Data Structures

### DBColumn

Represents a single database column with all its metadata:

```go
type DBColumn struct {
    // Identity
    ID          int32   // Unique column ID within discovery
    Schema      string  // Schema name (e.g., "public")
    Table       string  // Table name
    Name        string  // Column name
    Comment     string  // Column comment/description
    
    // Type Information
    Type        string  // SQL type (e.g., "integer", "text", "uuid")
    Array       bool    // Is this an array type?
    
    // Constraints
    NotNull     bool    // NOT NULL constraint
    PrimaryKey  bool    // Part of PRIMARY KEY
    UniqueKey   bool    // Has UNIQUE constraint
    
    // Full-Text Search
    FullText    bool    // Is tsvector type (PostgreSQL)
    
    // Foreign Key Information
    FKeySchema  string  // FK target schema
    FKeyTable   string  // FK target table
    FKeyCol     string  // FK target column
    FKRecursive bool    // Self-referential FK (same table)
    
    // Access Control
    Blocked     bool    // Is this column blocked from access?
}
```

### DBTable

Represents a database table:

```go
type DBTable struct {
    // Identity
    Schema       string      // Schema name
    Name         string      // Table name
    Comment      string      // Table comment
    Type         string      // "table", "view", "function", "virtual", "json"
    
    // Columns
    Columns      []DBColumn  // All columns in this table
    colMap       map[string]int  // Column name → index lookup
    
    // Special Columns
    PrimaryCol   DBColumn    // Primary key column
    SecondaryCol DBColumn    // Secondary column (for special cases)
    FullText     []DBColumn  // Full-text searchable columns
    
    // Access Control
    Blocked      bool        // Is this table blocked?
    
    // Function-backed Tables
    Func         DBFunction  // If Type="function", the backing function
}
```

### DBFunction

Represents a stored procedure or function:

```go
type DBFunction struct {
    // Identity
    Schema  string        // Schema name
    Name    string        // Function name
    Comment string        // Function comment
    
    // Signature
    Type    string        // Return type ("record", "integer", etc.)
    Agg     bool          // Is this an aggregate function?
    Inputs  []DBFuncParam // Input parameters
    Outputs []DBFuncParam // Output parameters (for record types)
}

type DBFuncParam struct {
    ID    int     // Parameter position
    Name  string  // Parameter name
    Type  string  // Parameter type
    Array bool    // Is array parameter?
}
```

### DBInfo

Container for all discovered database information:

```go
type DBInfo struct {
    // Database Identity
    Type    string  // "postgres" or "mysql"
    Version int     // Database version number
    Schema  string  // Current schema
    Name    string  // Database name
    
    // Content
    Tables    []DBTable           // All discovered tables
    Functions []DBFunction        // All discovered functions
    VTables   []VirtualTable      // Virtual/polymorphic tables
    
    // Lookups
    colMap    map[string]int      // "schema:table:column" → index
    tableMap  map[string]int      // "schema:table" → index
    hash      int                 // Schema hash for caching
}
```

## SQL Introspection Queries

### PostgreSQL Column Discovery

The PostgreSQL query (`sql/postgres_columns.sql`) introspects:

```sql
SELECT 
    n.nspname as "schema",          -- Schema name
    c.relname as "table",           -- Table name  
    f.attname AS "column",          -- Column name
    pg_catalog.format_type(...),    -- Data type with modifiers
    f.attnotnull AS not_null,       -- NOT NULL constraint
    
    -- Primary Key detection
    (CASE WHEN co.contype = 'p' THEN true ELSE false END) AS primary_key,
    
    -- Unique Key detection
    (CASE WHEN co.contype = 'u' THEN true ELSE false END) AS unique_key,
    
    -- Array detection (two methods)
    (CASE 
        WHEN f.attndims != 0 THEN true
        WHEN right(format_type(...), 2) = '[]' THEN true
        ELSE false
    END) AS is_array,
    
    -- Full-text search detection
    (CASE WHEN format_type(...) = 'tsvector' THEN TRUE ELSE FALSE END) AS full_text,
    
    -- Foreign Key extraction
    foreignkey_schema,  -- From pg_class join
    foreignkey_table,   -- From pg_class.relname
    foreignkey_column   -- From pg_attribute

FROM pg_attribute f
    JOIN pg_class c ON c.oid = f.attrelid
    LEFT JOIN pg_constraint co ON co.conrelid = c.oid AND f.attnum = ANY(co.conkey)
    -- ... more joins for FK resolution
    
WHERE c.relkind IN ('r', 'v', 'm', 'f', 'p')  -- tables, views, materialized, foreign, partitioned
    AND n.nspname NOT IN ('information_schema', 'pg_catalog')  -- skip system schemas
```

**Key PostgreSQL System Tables Used:**
- `pg_attribute`: Column definitions
- `pg_class`: Table/relation info
- `pg_namespace`: Schema info
- `pg_constraint`: Constraints (PK, FK, unique)
- `pg_attrdef`: Default values

### MySQL Column Discovery

MySQL requires a different approach due to its information_schema structure:

```sql
-- First query: Basic column info
SELECT 
    col.table_schema as "schema",
    col.table_name as "table",
    col.column_name as "column",
    col.data_type as "type",
    -- ... constraints from join with information_schema.statistics
FROM information_schema.columns col
    LEFT JOIN information_schema.statistics stat ON ...

UNION

-- Second query: Constraint info (PK, FK, Unique)
SELECT 
    kcu.table_schema, kcu.table_name, kcu.column_name,
    -- ... constraint types from table_constraints
FROM information_schema.key_column_usage kcu
    JOIN information_schema.table_constraints tc ON ...
```

**Why UNION?** MySQL's information_schema has limitations with JOINs in certain versions, so constraint info is queried separately and merged.

### Function Discovery

PostgreSQL functions:

```sql
SELECT 
    r.specific_name as func_id,
    r.routine_schema as func_schema,
    r.routine_name as func_name,
    r.data_type as data_type,        -- Return type
    p.ordinal_position as param_id,
    p.parameter_name as param_name,
    p.data_type as param_type,
    p.parameter_mode as param_kind   -- IN, OUT, INOUT

FROM information_schema.routines r
    RIGHT JOIN information_schema.parameters p ON ...
WHERE r.routine_type = 'FUNCTION'
```

## Discovery Process

### Step 1: GetDBInfo()

```go
func GetDBInfo(db *sql.DB, dbType string, blockList []string) (*DBInfo, error) {
    // Parallel execution for efficiency
    g := errgroup.Group{}
    
    // Query 1: Get database info
    g.Go(func() error {
        row := db.QueryRow(postgresInfo)  // or mysqlInfo
        return row.Scan(&dbVersion, &dbSchema, &dbName)
    })
    
    // Query 2: Get columns and functions
    g.Go(func() error {
        cols, err = DiscoverColumns(db, dbType, blockList)
        funcs, err = DiscoverFunctions(db, dbType, blockList)
        return err
    })
    
    g.Wait()
    
    return NewDBInfo(dbType, dbVersion, dbSchema, dbName, cols, funcs, blockList)
}
```

### Step 2: DiscoverColumns()

```go
func DiscoverColumns(db *sql.DB, dbtype string, blockList []string) ([]DBColumn, error) {
    // Select appropriate SQL
    var sqlStmt string
    switch dbtype {
    case "mysql":
        sqlStmt = mysqlColumnsStmt
    default:
        sqlStmt = postgresColumnsStmt
    }
    
    rows, err := db.Query(sqlStmt)
    
    // Use map to handle duplicate column entries (MySQL quirk)
    cmap := make(map[string]DBColumn)
    
    for rows.Next() {
        var c DBColumn
        rows.Scan(&c.Schema, &c.Table, &c.Name, &c.Type, ...)
        
        k := (c.Schema + ":" + c.Table + ":" + c.Name)
        
        // Merge info if column already seen
        if v, ok := cmap[k]; ok {
            // Merge constraints (PK, FK, etc.)
            if c.PrimaryKey { v.PrimaryKey = true }
            // ... more merging
            cmap[k] = v
        } else {
            cmap[k] = c
        }
    }
    
    // Convert map to slice
    var cols []DBColumn
    for _, c := range cmap {
        cols = append(cols, c)
    }
    return cols, nil
}
```

### Step 3: NewDBInfo()

```go
func NewDBInfo(...) *DBInfo {
    di := &DBInfo{
        Type: dbType, Version: dbVersion, Schema: dbSchema, Name: dbName,
        colMap: make(map[string]int),
        tableMap: make(map[string]int),
    }
    
    // Group columns by table
    type st struct { schema, table string }
    tm := make(map[st][]DBColumn)
    
    for _, c := range cols {
        k := st{c.Schema, c.Table}
        tm[k] = append(tm[k], c)
    }
    
    // Create DBTable for each group
    for k, tcols := range tm {
        ti := NewDBTable(k.schema, k.table, "", tcols)
        
        // Skip internal tables
        if strings.HasPrefix(ti.Name, "_gj_") {
            continue
        }
        
        ti.Blocked = isInList(ti.Name, blockList)
        di.AddTable(ti)
    }
    
    // Add function-backed tables
    for _, f := range funcs {
        if f.Type == "record" && len(f.Outputs) > 0 {
            // Create table from function outputs
            cols := functionOutputsToColumns(f)
            t := NewDBTable(f.Schema, f.Name, "function", cols)
            t.Func = f
            di.AddTable(t)
        }
    }
    
    return di
}
```

### Step 4: NewDBTable()

```go
func NewDBTable(schema, name, _type string, cols []DBColumn) DBTable {
    ti := DBTable{
        Schema:  schema,
        Name:    name,
        Type:    _type,
        Columns: cols,
        colMap:  make(map[string]int, len(cols)),
    }
    
    for i, c := range cols {
        // Set schema/table on column
        cols[i].Schema = schema
        cols[i].Table = name
        
        // Identify special columns
        switch {
        case c.FullText:
            ti.FullText = append(ti.FullText, c)
        case c.PrimaryKey:
            ti.PrimaryCol = c
        }
        
        // Build column lookup
        ti.colMap[c.Name] = i
    }
    return ti
}
```

## Special Cases

### Recursive/Self-Referential Foreign Keys

Detected when FK target is the same table:

```go
if v.FKeySchema == v.Schema && v.FKeyTable == v.Table {
    v.FKRecursive = true
}
```

Example: `employees.manager_id → employees.id`

### Virtual/Polymorphic Tables

For relationships like:
```
comments (commentable_id, commentable_type) → posts OR users
```

Virtual tables are configured, not discovered:

```go
type VirtualTable struct {
    Name       string  // "commentable"
    IDColumn   string  // "commentable_id"
    TypeColumn string  // "commentable_type"
    FKeyColumn string  // Target ID column
}
```

### Function-Backed Tables

PostgreSQL functions returning records become queryable tables:

```sql
CREATE FUNCTION get_user_stats(user_id int)
RETURNS TABLE(post_count int, comment_count int) AS $$
...
$$ LANGUAGE plpgsql;
```

These are discovered via `information_schema.routines` and their output parameters become columns.

### Blocked Tables/Columns

The `blockList` parameter supports regex patterns:

```go
func isInList(val string, s []string) bool {
    for _, v := range s {
        regex := fmt.Sprintf("^%s$", v)
        if matched, _ := regexp.MatchString(regex, val); matched {
            return true
        }
    }
    return false
}
```

Example blocklist: `["_audit$", "^pg_", "password"]`

## Schema Caching

A hash is computed for cache invalidation:

```go
h := fnv.New128()
hv := fmt.Sprintf("%s%d%s%s", dbType, dbVersion, dbSchema, dbName)
h.Write([]byte(hv))

for _, c := range cols {
    h.Write([]byte(c.String()))
}
for _, fn := range funcs {
    h.Write([]byte(fn.String()))
}

di.hash = h.Size()
```

## Extension Points for Custom DSL

### 1. Additional Database Support

Add new SQL files and detection logic:
- `sqlite_columns.sql`
- `cockroachdb_columns.sql`

### 2. Custom Type Mapping

Current type detection is basic. Add:
- JSON schema extraction from JSON columns
- Custom type aliases
- Generated column detection

### 3. Enhanced Metadata

Capture additional info:
- Column comments/descriptions
- Default values
- Check constraints
- Indexes (for optimization hints)

### 4. Schema Diffing

Add capability to detect schema changes:
```go
func (di *DBInfo) Diff(other *DBInfo) []SchemaChange
```

### 5. Multi-Schema Support

Currently single-schema focused. Add:
- Cross-schema FK resolution
- Schema qualification in queries