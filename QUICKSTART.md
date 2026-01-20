# Quick Start Guide

Get started with GraphJin's schema introspection and auto-join algorithm in under 5 minutes.

## Installation

```bash
cd extracted
go mod download
```

## Basic Example

```go
package main

import (
    "database/sql"
    "fmt"
    "log"
    
    _ "github.com/lib/pq"
    "github.com/yourusername/graphjin-extracted/schema"
)

func main() {
    // 1. Connect to database
    db, err := sql.Open("postgres", 
        "postgres://user:password@localhost:5432/mydb?sslmode=disable")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // 2. Discover schema
    dbInfo, err := schema.GetDBInfo(db, "postgres", nil)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Found %d tables\n", len(dbInfo.Tables))
    
    // 3. Build relationship graph
    dbSchema, err := schema.NewDBSchema(dbInfo, nil)
    if err != nil {
        log.Fatal(err)
    }
    
    // 4. Find join path
    paths, err := dbSchema.FindPath("orders", "customers", "")
    if err != nil {
        log.Fatal(err)
    }
    
    // 5. Use the relationship
    if len(paths) > 0 {
        p := paths[0]
        fmt.Printf("JOIN: %s.%s = %s.%s\n",
            p.LT.Name, p.LC.Name,
            p.RT.Name, p.RC.Name)
    }
}
```

## Common Use Cases

### 1. Inspect All Tables

```go
dbInfo, _ := schema.GetDBInfo(db, "postgres", nil)

for _, table := range dbInfo.Tables {
    fmt.Printf("Table: %s.%s\n", table.Schema, table.Name)
    for _, col := range table.Columns {
        fmt.Printf("  - %s (%s)\n", col.Name, col.Type)
        if col.FKeyTable != "" {
            fmt.Printf("    FK -> %s.%s\n", col.FKeyTable, col.FKeyCol)
        }
    }
}
```

### 2. Find Related Tables

```go
dbSchema, _ := schema.NewDBSchema(dbInfo, nil)

// Find all tables directly related to "users"
table, _ := dbSchema.Find("public", "users")
related, _ := dbSchema.GetFirstDegree(table)

for _, rel := range related {
    fmt.Printf("Related: %s via %s\n", rel.Table.Name, rel.Name)
}
```

### 3. Find Complex Join Path

```go
// Find path from orders -> products
// going through order_items table
paths, err := dbSchema.FindPath("orders", "products", "order_items")
if err != nil {
    log.Fatal(err)
}

for i, path := range paths {
    fmt.Printf("Path %d: %s.%s -> %s.%s\n", i+1,
        path.LT.Name, path.LC.Name,
        path.RT.Name, path.RC.Name)
}
```

### 4. Exclude Tables with Blocklist

```go
blockList := []string{
    "migrations",
    "internal_*",    // Wildcard: exclude tables starting with "internal_"
    "*.password",    // Exclude password column from all tables
}

dbInfo, err := schema.GetDBInfo(db, "postgres", blockList)
```

### 5. Use Table Aliases

```go
aliases := map[string][]string{
    "users": {"authors", "creators", "members"},
    "posts": {"articles", "content"},
}

dbSchema, _ := schema.NewDBSchema(dbInfo, aliases)

// Now all these work:
dbSchema.FindPath("posts", "users", "")
dbSchema.FindPath("articles", "authors", "")
dbSchema.FindPath("content", "creators", "")
```

## Database Support

### PostgreSQL

```go
db, _ := sql.Open("postgres", "postgres://user:pass@host:5432/db")
dbInfo, _ := schema.GetDBInfo(db, "postgres", nil)
```

### MySQL

```go
db, _ := sql.Open("mysql", "user:pass@tcp(host:3306)/db")
dbInfo, _ := schema.GetDBInfo(db, "mysql", nil)
```

## Understanding Relationships

The library automatically detects these relationship types:

| Type | Description | Example |
|------|-------------|---------|
| `RelOneToMany` | Standard FK | `posts.user_id -> users.id` |
| `RelOneToOne` | FK with unique constraint | `profiles.user_id -> users.id` (unique) |
| `RelRecursive` | Self-referential | `comments.parent_id -> comments.id` |
| `RelPolymorphic` | Type + ID pattern | `taggable_type + taggable_id` |
| `RelEmbedded` | JSON/JSONB | Embedded document relationship |

## Error Handling

```go
paths, err := dbSchema.FindPath("table1", "table2", "")
switch err {
case schema.ErrFromEdgeNotFound:
    fmt.Println("Source table has no relationships")
case schema.ErrToEdgeNotFound:
    fmt.Println("Target table has no relationships")
case schema.ErrPathNotFound:
    fmt.Println("No path exists between tables")
default:
    if err != nil {
        log.Fatal(err)
    }
}
```

## Path Selection

When multiple paths exist, the algorithm returns them sorted by weight:

```go
paths, _ := dbSchema.FindPath("comments", "users", "")

// paths[0] = Direct path (lowest weight)
// paths[1] = Indirect path through posts
// paths[2] = Longer indirect path

// Use the first path (optimal)
bestPath := paths[0]
```

## Performance Tips

1. **Cache DBSchema**: Build once, reuse many times
   ```go
   var dbSchema *schema.DBSchema
   
   func init() {
       db, _ := sql.Open(...)
       dbInfo, _ := schema.GetDBInfo(db, "postgres", nil)
       dbSchema, _ = schema.NewDBSchema(dbInfo, nil)
   }
   ```

2. **Use blocklist**: Reduce graph size
   ```go
   blockList := []string{"logs_*", "temp_*", "archive_*"}
   ```

3. **Limit discovered tables**: Only introspect what you need
   ```go
   // Future: schema-specific discovery
   // Currently discovers all tables in database
   ```

## Testing

Run the included test:

```bash
cd extracted
go test -v
```

Output shows example usage with test data.

## Next Steps

- Read [README.md](README.md) for detailed architecture
- Check [SCHEMA_INTROSPECTION.md](extracted/schema/SCHEMA_INTROSPECTION.md) for schema discovery details
- Check [AUTOJOIN_ALGORITHM.md](extracted/autojoin/AUTOJOIN_ALGORITHM.md) for pathfinding algorithm details
- Review [FILES.md](FILES.md) for complete file reference

## Common Patterns

### Pattern 1: REST API with Auto-Join

```go
// GET /api/posts?include=author,comments.user
func handler(w http.ResponseWriter, r *http.Request) {
    includes := r.URL.Query().Get("include")
    
    for _, inc := range strings.Split(includes, ",") {
        parts := strings.Split(inc, ".")
        from := "posts"
        to := parts[len(parts)-1]
        
        paths, _ := dbSchema.FindPath(from, to, "")
        // Generate SQL JOIN from paths
    }
}
```

### Pattern 2: GraphQL Resolver

```go
func (r *postResolver) Author(ctx context.Context, obj *Post) (*User, error) {
    paths, err := dbSchema.FindPath("posts", "users", "")
    if err != nil {
        return nil, err
    }
    
    rel := schema.PathToRel(paths[0])
    // Use rel.LC.Name (posts.user_id) to fetch author
}
```

### Pattern 3: Query Builder

```go
type QueryBuilder struct {
    schema *schema.DBSchema
}

func (qb *QueryBuilder) Join(from, to string) string {
    paths, _ := qb.schema.FindPath(from, to, "")
    if len(paths) == 0 {
        return ""
    }
    
    p := paths[0]
    return fmt.Sprintf("JOIN %s ON %s.%s = %s.%s",
        p.RT.Name,
        p.LT.Name, p.LC.Name,
        p.RT.Name, p.RC.Name)
}
```

## Troubleshooting

**Q: "table not found" error**
```go
// Make sure to use the correct schema
table, err := dbSchema.Find("public", "users")  // PostgreSQL
table, err := dbSchema.Find("mydb", "users")    // MySQL
```

**Q: No paths found between tables**
```go
// Check if foreign keys exist in database
// The introspection depends on actual FK constraints
```

**Q: Performance issues with large databases**
```go
// Use aggressive blocklisting
blockList := []string{
    "audit_*",
    "log_*", 
    "temp_*",
    "*.created_at",  // Exclude timestamp columns
}
```

## License

Apache License 2.0 (inherited from GraphJin)