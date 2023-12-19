package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type User struct {
	Host string
	Name string
}

func isUserInIgnoreList(host, user, ignore string) bool {
	if ignore == "" {
		return false
	}

	usersToIgnore := strings.Split(ignore, ",")
	for _, u := range usersToIgnore {
		parts := strings.Split(u, "@")
		if len(parts) > 1 {
			// 检查 host 和 user 是否与分割后的第二个元素和第一个元素相同
			if strings.TrimSpace(parts[1]) == host && strings.TrimSpace(parts[0]) == user {
				return true
			}
		} else {
			// 只检查 user 是否相同
			if strings.TrimSpace(parts[0]) == user {
				return true
			}
		}
	}
	return false
}

func printUserInfo(host, port, user, password, ignore string) {
	// Constructing the connection string
	connectionString := fmt.Sprintf("%s:%s@tcp(%s:%s)/mysql", user, password, host, port)
	// Open a connection to MySQL
	db, err := sql.Open("mysql", connectionString)
	if err != nil {
		log.Fatalf("Error connecting to MySQL: %v", err)
	}
	defer db.Close()

	// Begin a transaction
	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("Error starting transaction: %v", err)
	}
	defer tx.Rollback() // Rollback the transaction if not committed
	versionInfo, err := tx.Query("SELECT VERSION()")
	if err != nil {
		log.Fatalf("Error getting MySQL version: %v", err)
	}
	var version string
	for versionInfo.Next() {
		if err := versionInfo.Scan(&version); err != nil {
			log.Fatal(err)
		}
	}
	versionParts := strings.Split(version, ".")
	majorVersion, err := strconv.Atoi(versionParts[0])
	minorVersion, err := strconv.Atoi(versionParts[1])
	patchVersion, err := strconv.Atoi(strings.Split(versionParts[2], "-")[0])
	// Set session variable if MySQL version >= 8.0.17
	if majorVersion > 8 || (majorVersion == 8 && minorVersion > 0) || (majorVersion == 8 && minorVersion == 0 && patchVersion >= 17) {
		_, err := tx.Exec("SET session print_identified_with_as_hex = on")
		if err != nil {
			log.Fatalf("Error setting session variable: %v", err)
		}
	}
	// Query user information excluding specific users
	rows, err := tx.Query("SELECT Host, User FROM mysql.user WHERE user NOT IN ('mysql.infoschema','mysql.session','mysql.sys')")
	if err != nil {
		log.Fatalf("Error querying user information: %v", err)
	}

	var users []User

	// 遍历整个结果集
	for rows.Next() {
		var user User
		err := rows.Scan(&user.Host, &user.Name)
		if err != nil {
			log.Fatal(err)
		}

		// 将结果添加到切片中
		users = append(users, user)
	}
	// Iterate through the result set
	for _, u := range users {
		host, user := u.Host, u.Name
		// Query to show the CREATE USER statement
		if isUserInIgnoreList(host, user, ignore) {
			continue
		}
		showCreateUserQuery := fmt.Sprintf("SHOW CREATE USER '%s'@'%s'", user, host)
		createUserRows, err := tx.Query(showCreateUserQuery)
		if err != nil {
			log.Printf("Error executing query: %v", err)
			continue
		}

		// Iterate through the result set and print the CREATE USER statement
		for createUserRows.Next() {
			var createUserStmt string
			createUserRows.Scan(&createUserStmt)
			fmt.Printf("%s;\n", createUserStmt)
		}

		// Query to show the GRANTS for the user
		showGrantsQuery := fmt.Sprintf("SHOW GRANTS FOR '%s'@'%s'", user, host)
		grantsRows, err := tx.Query(showGrantsQuery)
		if err != nil {
			log.Printf("Error executing query: %v", err)
			continue
		}

		// Iterate through the result set and print the GRANTS statements
		for grantsRows.Next() {
			var grantsStmt string
			grantsRows.Scan(&grantsStmt)
			fmt.Printf("%s;\n", grantsStmt)
		}
		fmt.Println() // Add a newline between users
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		log.Fatalf("Error committing transaction: %v", err)
	}
}

func main() {
	// Parse command line arguments
	host := flag.String("h", "localhost", "MySQL host")
	port := flag.Int("P", 3306, "MySQL port")
	user := flag.String("u", "", "MySQL user")
	password := flag.String("p", "", "MySQL password")
	ignore := flag.String("ignore", "", "Ignore specific users (comma-separated)")
	flag.Parse()

	// Call the function to print user information
	printUserInfo(*host, fmt.Sprintf("%d", *port), *user, *password, *ignore)
}
