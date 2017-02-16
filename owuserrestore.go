package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	userID := flag.Int("userid", 0, "user ID to be restored")
	dbOldUser := flag.String("dbolduser", "root", "username for old database")
	dbOldPass := flag.String("dboldpass", "root", "password for old database")
	dbOldName := flag.String("dboldname", "owuserrestoreold", "name of old database")
	dbCurrentUser := flag.String("dbcurrentuser", "root", "username for current database")
	dbCurrentPass := flag.String("dbcurrentpass", "root", "password for current database")
	dbCurrentName := flag.String("dbcurrentname", "owuserrestorecurrent", "name of current database")
	dbMergeUser := flag.String("dbmergeuser", "root", "username for merge database")
	dbMergePass := flag.String("dbmergepass", "root", "password for merge database")
	dbMergeName := flag.String("dbmergename", "owuserrestoremerge", "name of merge database")
	realThing := flag.Bool("real", false, "actually run the merge, don't just print statistics")

	flag.Parse()

	if *userID <= 0 {
		fmt.Println(userID)
		log.Fatal("Error: Valid user ID required!")
	}

	dbOld, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/%s", *dbOldUser, *dbOldPass, *dbOldName))
	checkErr(err)

	dbCurrent, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/%s", *dbCurrentUser, *dbCurrentPass, *dbCurrentName))
	checkErr(err)

	dbMerge, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/%s", *dbMergeUser, *dbMergePass, *dbMergeName))
	checkErr(err)

	if !*realThing {
		fmt.Println("Just stats, merge database won't be touched...")
		fmt.Println()
		printUserIdTables(dbOld, *dbOldName)
		printUserIdRecordCounts(*userID, *dbOldName, dbOld, dbCurrent)
		printDeletedRecords(*userID, *dbOldName, dbOld, dbCurrent)
	} else {
		fmt.Println("Merging!")
		fmt.Println()
		printUserIdTables(dbOld, *dbOldName)
		printUserIdRecordCounts(*userID, *dbOldName, dbOld, dbCurrent)
		missingRecords := printDeletedRecords(*userID, *dbOldName, dbOld, dbCurrent)
		restoreMissingRecords(missingRecords, *userID, dbOld, dbMerge)
	}
}

func printUserIdTables(db *sql.DB, dbName string) {
	fmt.Printf("The following tables in database '%s' contain the field 'userId':\n", dbName)

	tableNames := findTableNamesByColumn("userId", dbName, db)

	for _, tableName := range tableNames {
		fmt.Println(tableName)
	}

	fmt.Printf("Total %d tables.\n", len(tableNames))
	fmt.Println()
}

func printUserIdRecordCounts(userID int, dbOldName string, dbOld *sql.DB, dbCurrent *sql.DB) {
	fmt.Printf("The following tables contain data for user ID %d:\n", userID)

	tableNames := findTableNamesByColumn("userId", dbOldName, dbOld)

	var different, identical, zeroed, reduced, diffTotal int

	for _, tableName := range tableNames {
		var countAlt, countNeu int

		err := dbOld.QueryRow("SELECT COUNT(*) FROM "+tableName+" WHERE userId = ?", userID).Scan(&countAlt)
		checkErr(err)

		err = dbCurrent.QueryRow("SELECT COUNT(*) FROM "+tableName+" WHERE userId = ?", userID).Scan(&countNeu)
		checkErr(err)

		if countAlt > 0 {
			if countAlt != countNeu {
				fmt.Printf("%s: %d -> %d (number of records in old -> current database)\n", tableName, countAlt, countNeu)
				different++
			} else {
				fmt.Printf("%s: %d (same number of records in both databases)\n", tableName, countAlt)
				identical++
			}
			if countAlt > 0 && countNeu == 0 {
				zeroed++
			}
			if countAlt > 0 && countNeu > 0 && countAlt > countNeu {
				reduced++
			}
			diffTotal = diffTotal + (countAlt - countNeu)
		}
	}

	fmt.Printf("Number of tables containing user's data: %d\n", different+identical)
	fmt.Printf("Number of tables in which the number of records is identical: %d\n", identical)
	fmt.Printf("Number of tables in which the number of records is different: %d\n", different)
	fmt.Printf("Number of tables which contained data in the old database, but are empty in the current database: %d\n", zeroed)
	fmt.Printf("Number of tables which contained data in the old database, but contain fewer records in the current database: %d\n", reduced)
	fmt.Printf("Number of records missing in the current database: %d\n", diffTotal)
	fmt.Println()
}

func printDeletedRecords(userID int, dbOldName string, dbOld *sql.DB, dbCurrent *sql.DB) map[string][]int {
	fmt.Printf("IDs of missing records by table:\n")

	missingRecords := make(map[string][]int, 0)

	tableNames := findTableNamesByColumn("userId", dbOldName, dbOld)
	for _, tableName := range tableNames {

		oldIDs := make([]int, 0)
		newIDs := make([]int, 0)

		rows, err := dbOld.Query("SELECT id FROM "+tableName+" WHERE userId = ?", userID)
		checkErr(err)
		for rows.Next() {
			var id int
			err := rows.Scan(&id)
			checkErr(err)
			oldIDs = append(oldIDs, id)
		}
		rows.Close()

		rows, err = dbCurrent.Query("SELECT id FROM "+tableName+" WHERE userId = ?", userID)
		checkErr(err)
		for rows.Next() {
			var id int
			err := rows.Scan(&id)
			checkErr(err)
			newIDs = append(newIDs, id)
		}
		rows.Close()

		missingIDs := make([]int, 0)
		for _, oldID := range oldIDs {
			if !intInSlice(oldID, newIDs) {
				missingIDs = append(missingIDs, oldID)
			}
		}

		if len(oldIDs) > len(newIDs) {
			missingRecords[tableName] = missingIDs
			fmt.Printf("%s: %v\n", tableName, missingIDs)
		}
	}

	fmt.Println()

	return missingRecords
}

func restoreMissingRecords(missingRecords map[string][]int, userID int, dbOld *sql.DB, dbMerge *sql.DB) {
	fmt.Printf("Merge begins:\n")

	missingRecords["ow_base_user"] = []int{userID}

	for tableName, IDs := range missingRecords {
		fmt.Printf("Merging table %s...\n", tableName)
		for _, id := range IDs {
			fmt.Printf("  record with ID %d\n", id)

			rows, err := dbOld.Query("SELECT * FROM "+tableName+" WHERE id = ?", id)
			checkErr(err)

			columns, err := rows.Columns()
			checkErr(err)

			colNum := len(columns)

			if !rows.Next() {
				log.Fatal("Failed on rows.Next()")
			}

			container := make([]interface{}, colNum)
			values := make([]interface{}, colNum)

			for i := range container {
				values[i] = &container[i]
			}

			err = rows.Scan(values...)
			checkErr(err)

			rows.Close()

			record := make(map[string]interface{})
			for i, colName := range columns {
				value := values[i].(*interface{})
				record[colName] = *value
			}

			insert := "INSERT INTO " + tableName + " ("
			for i, columnName := range columns {
				if i > 0 {
					insert = insert + ","
				}
				insert = insert + "`" + columnName + "`"
			}
			insert = insert + ") VALUES ("
			for i := range columns {
				if i > 0 {
					insert = insert + ","
				}
				insert = insert + "?"
			}
			insert = insert + ") "

			stmt, err := dbMerge.Prepare(insert)
			checkErr(err)

			_, err = stmt.Exec(values...)
			checkErr(err)

			stmt.Close()
		}
	}

	fmt.Printf("Merge successful!\n")
	fmt.Println()
}

func findTableNamesByColumn(column string, dbName string, db *sql.DB) []string {
	rows, err := db.Query("SELECT DISTINCT TABLE_NAME FROM INFORMATION_SCHEMA.COLUMNS WHERE COLUMN_NAME = ? AND TABLE_SCHEMA = ?", column, dbName)
	checkErr(err)
	defer rows.Close()

	var tableName string
	tableNames := make([]string, 0)

	for rows.Next() {
		err := rows.Scan(&tableName)
		checkErr(err)
		tableNames = append(tableNames, tableName)
	}

	return tableNames
}

func intInSlice(lookup int, slice []int) bool {
	for _, value := range slice {
		if value == lookup {
			return true
		}
	}
	return false
}

func checkErr(e error) {
	if e != nil {
		log.Println("An error occurred! Stopped half-way through...")
		log.Fatal(e)
	}
}
