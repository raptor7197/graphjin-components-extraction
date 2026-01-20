# Auto-Join Algorithm Deep Dive

## Overview

The auto-join algorithm in GraphJin automatically discovers and generates SQL JOINs between tables based on foreign key relationships. This eliminates the need for developers to manually specify join conditions in GraphQL queries.

## Core Concept: Relationship Graph

The algorithm builds a **weighted directed graph** where:
- **Nodes** = Database tables
- **Edges** = Foreign key relationships (bidirectional)
- **Weights** = Priority/cost of using that relationship

```
┌─────────┐     FK: user_id      ┌─────────┐
│  users  │◄────────────────────│  posts  │
│ (id=0)  │     weight=1         │ (id=1)  │
└─────────┘                      └─────────┘
     ▲                                │
     │  FK: user_id                   │ FK: post_id
     │  weight=1                      │ weight=1
     │                                ▼
     │                          ┌──────────┐
     └──────────────────────────│ comments │
                FK: user_id     │  (id=2)  │
                weight=1        └──────────┘
```

## Algorithm Phases

### Phase 1: Graph Construction (Build Time)

When the schema is loaded, `addToGraph()` is called for each foreign key:

```go
// For posts.user_id → users.id

// 1. Create forward edge (posts → users)
e1 := TEdge{
    From:   1,              // posts node ID
    To:     0,              // users node ID
    Weight: 1,              // Standard FK weight
    Type:   RelOneToMany,
    LT:     posts,          // Left table
    RT:     users,          // Right table
    L:      user_id_col,    // Left column (FK column)
    R:      id_col,         // Right column (PK column)
    CName:  "user_id",      // Column name
}

// 2. Create reverse edge (users → posts)
e2 := TEdge{
    From:   0,              // users node ID
    To:     1,              // posts node ID
    Weight: 1,
    Type:   RelOneToOne,    // Reverse is one-to-one from PK side
    LT:     users,
    RT:     posts,
    L:      id_col,
    R:      user_id_col,
    CName:  "user_id",
}
```

#### Edge Indexing Strategy

Edges are indexed multiple ways for fast lookup:

```go
// By table name
edgesIndex["posts"] = [{nodeID: 1, edgeIDs: [0, 1, ...]}]
edgesIndex["users"] = [{nodeID: 0, edgeIDs: [2, 3, ...]}]

// By relationship name (column name with _id stripped)
edgesIndex["user"]  = [{nodeID: 0, edgeIDs: [...]}]  // From "user_id"
edgesIndex["post"]  = [{nodeID: 1, edgeIDs: [...]}]  // From "post_id"

// By full table name (as fallback)
edgesIndex["users"] = [{nodeID: 0, edgeIDs: [...]}]
```

### Phase 2: Path Discovery (Query Time)

When a GraphQL query like this is parsed:

```graphql
query {
  comments {
    body
    user {      # Need to find path: comments → users
      name
    }
  }
}
```

The `FindPath("user", "comments", "")` is called:

```go
func (s *DBSchema) FindPath(from, to, through string) ([]TPath, error) {
    // 1. Lookup edge indices
    fl, ok := s.edgesIndex[from]  // "user" → users table edges
    tl, ok := s.edgesIndex[to]    // "comments" → comments table edges
    
    // 2. Find paths between all combinations
    res, err := s.between(fl, tl, through)
    
    // 3. Convert edges to TPath result
    path := []TPath{}
    for _, eid := range res.edges {
        edge := s.allEdges[eid]
        path = append(path, TPath{...})
    }
    return path, nil
}
```

### Phase 3: All-Paths Algorithm

The `AllPaths()` function finds all possible paths between two nodes:

```go
func (g *Graph) AllPaths(from, to int32) [][]int32 {
    var paths [][]int32
    var limit int
    
    // Min-heap ordered by path weight
    h := newHeap()
    h.push(path{weight: 0, parent: from, nodes: []int32{from}})
    
    // Track visited [parent, node] pairs to avoid cycles
    visited := make(map[[2]int32]struct{})
    
    for len(*h.paths) > 0 {
        // Safety limit
        if limit > 3000 {
            return paths
        }
        limit++
        
        // Pop minimum weight path
        p := h.pop()
        pnode := p.nodes[len(p.nodes)-1]
        
        // Skip if already visited this transition
        if _, ok := visited[[2]int32{p.parent, pnode}]; ok {
            continue
        }
        
        // Found destination?
        if pnode == to && len(p.nodes) > 1 {
            // Check for duplicate paths
            for _, v := range paths {
                if equals(v, p.nodes) {
                    return paths
                }
            }
            paths = append(paths, p.nodes)
            continue
        }
        
        // Explore neighbors
        for _, neighbor := range g.graph[pnode] {
            // Skip if already in path (unless it's the destination)
            if _, ok := p.visited[neighbor]; ok && neighbor != to {
                continue
            }
            
            // Create new path with accumulated weight
            p1 := path{
                weight:  p.weight + 1,
                parent:  pnode,
                nodes:   append([]int32{}, p.nodes...),
                visited: make(map[int32]struct{}),
            }
            p1.nodes = append(p1.nodes, neighbor)
            for _, v := range p1.nodes {
                p1.visited[v] = struct{}{}
            }
            h.push(p1)
        }
    }
    return paths
}
```

### Phase 4: Edge Selection

Given multiple paths, `pickEdges()` selects the optimal edges:

```go
func (s *DBSchema) pickEdges(path []int32, from, to edgeInfo) (edges []int32, allFound bool) {
    pathLen := len(path)
    peID := int32(-2)  // Previous edge ID (avoid backtracking)
    
    for i := 1; i < pathLen; i++ {
        fn := path[i-1]  // From node
        tn := path[i]    // To node
        lines := s.relationshipGraph.GetEdges(fn, tn)
        
        switch {
        case i == 1:
            // First edge: Must match 'from' edge info
            if v := pickLine(lines, from, peID); v != nil {
                edges = append(edges, v.ID)
                peID = v.ID
            } else {
                return  // Path not valid
            }
            
        case i == (pathLen - 1):
            // Last edge: Prefer matching 'to' edge info
            if v := pickLine(lines, to, peID); v != nil {
                edges = append(edges, v.ID)
            } else {
                // Fall back to minimum weight
                v := minWeightedLine(lines, peID)
                edges = append(edges, v.ID)
            }
            
        default:
            // Middle edges: Pick minimum weight
            v := minWeightedLine(lines, peID)
            edges = append(edges, v.ID)
            peID = v.ID
        }
    }
    allFound = true
    return
}
```

## Edge Weight Strategy

Weights control path preference when multiple routes exist:

| Relationship Type | Weight | Reason |
|------------------|--------|--------|
| Regular FK       | 1      | Most common, preferred |
| Embedded JSON    | 5      | Requires JSON parsing |
| Remote API       | 8      | External call overhead |
| Recursive        | 10     | May cause deep recursion |
| Polymorphic      | 15     | Requires type checking |

Lower weight = Higher preference

## The "Through" Parameter

The `through` parameter forces paths through a specific table:

```go
func (s *DBSchema) pickThroughPath(paths [][]int32, through string) ([][]int32, error) {
    // Find the node ID for 'through' table
    v, ok := s.tindex[(s.DBSchema() + ":" + through)]
    if !ok {
        return nil, ErrThoughNodeNotFound
    }
    
    // Filter paths that include this node
    var npaths [][]int32
    for i := range paths {
        for j := range paths[i] {
            if paths[i][j] == v.nodeID {
                npaths = append(npaths, paths[i])
            }
        }
    }
    return npaths, nil
}
```

Example:
```graphql
query {
  users {
    # Without through: users.id = comments.user_id (direct)
    # With through: users → posts → comments
    comments(through: "posts") {
      body
    }
  }
}
```

## Relationship Name Resolution

The algorithm tries multiple naming patterns to match GraphQL fields to tables:

```go
// Column: author_id
// Tries:
//   1. "author" (strip _id suffix)
//   2. "author_id" (exact match)
//   3. Table name directly

func GetRelName(colName string) string {
    cn := colName
    
    if strings.HasSuffix(cn, "_id") {
        return colName[:len(colName)-3]  // "author_id" → "author"
    }
    if strings.HasSuffix(cn, "_ids") {
        return colName[:len(colName)-4]  // "author_ids" → "author"
    }
    if strings.HasPrefix(cn, "id_") {
        return colName[3:]  // "id_author" → "author"
    }
    if strings.HasPrefix(cn, "ids_") {
        return colName[4:]  // "ids_author" → "author"
    }
    return cn
}
```

## Result Structure

The path result contains everything needed to generate a JOIN:

```go
type TPath struct {
    Rel RelType    // OneToOne, OneToMany, etc.
    LT  DBTable    // Left table info
    LC  DBColumn   // Left column (typically FK)
    RT  DBTable    // Right table info
    RC  DBColumn   // Right column (typically PK)
}

// Converted to DBRel for query generation:
type DBRel struct {
    Type  RelType
    Left  DBRelLeft{Ti: table, Col: column}
    Right DBRelRight{Ti: table, Col: column}
}
```

## Complexity Analysis

- **Graph Construction**: O(E) where E = number of foreign keys
- **Path Finding**: O(V + E) with 3000 iteration limit
- **Edge Selection**: O(P × E) where P = path length

The iteration limit prevents exponential blowup in highly connected schemas.

## Limitations & Considerations

1. **No Many-to-Many Detection**: Join tables must be explicitly queried
2. **Single Schema**: Cross-schema relationships need explicit handling
3. **No Composite Keys**: Only single-column FKs are auto-detected
4. **Weight Customization**: Currently hard-coded in source
5. **Path Caching**: Paths are recalculated on each query (potential optimization)

## Extension Points for Custom DSL

1. **Custom Weight Configuration**: Allow runtime weight adjustment
2. **Path Annotations**: Add metadata to paths for custom JOIN generation
3. **Relationship Plugins**: Hook system for custom relationship types
4. **Multi-Path Results**: Return all paths ranked by cost
5. **Path Constraints**: Max hops, excluded tables, required intermediates