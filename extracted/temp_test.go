package extracted

import (
	"database/sql"
	_ "github.com/lib/pq" // postgres driver

	"github.com/yourusername/graphjin-extracted/schema"
)

func TestUsageExample() {
	// 1. Discover schema
	// For demonstration purposes, we'll use a dummy DBInfo.
	// In a real scenario, you'd connect to a database:
	// db, err := sql.Open("postgres", "user=postgres password=postgres dbname=exampledb sslmode=disable")
	// if err != nil {
	// 	panic(err)
	// }
	// defer db.Close()
	// dbInfo, err := schema.GetDBInfo(db, "postgres", nil)
	// if err != nil {
	// 	panic(err)
	// }

	// Using the test DBInfo provided in the schema package
	dbInfo := schema.GetTestDBInfo()

	// 2. Build relationship graph
	dbSchema, err := schema.NewDBSchema(dbInfo, nil)
	if err != nil {
		panic(err)
	}

	// 3. Find path between tables
	path, err := dbSchema.FindPath("comments", "users", "")
	if err != nil {
		panic(err)
	}

	// 4. Use relationship info
	rel := schema.PathToRel(path[0])

	// For a real test, you'd assert on the values of rel
	// For this example, we'll just print them.
	println("Relationship Type:", rel.Type)
	println("Left Table:", rel.LT.Name, "Column:", rel.LC.Name)
	println("Right Table:", rel.RT.Name, "Column:", rel.RC.Name)
}
