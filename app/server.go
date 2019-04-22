package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"net/http"
	"os"
)

// Sentences represents the Lines table in the database
type Sentences struct {
	Sentence string `json:"sentence"`
}

func main() {

	// The landing page for the application. Useful for showing the generated credentials
	http.HandleFunc("/sentence", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "App Running!\n\n")

		fmt.Fprintf(w, "Loaded Environment Variables:\n")
		fmt.Fprintf(w, "db_username: %s\n", os.Getenv("DB_USERNAME"))
		fmt.Fprintf(w, "db_password: %s\n", os.Getenv("DB_PASSWORD"))
		fmt.Fprintf(w, "db_host: %s\n", os.Getenv("DB_HOST"))
		fmt.Fprintf(w, "db_port: %s\n", os.Getenv("DB_PORT"))
	})

	// This endpoint is just a little helper endpoint that creates a new database/table
	// on a freshly provisioned database instance
	http.HandleFunc("/sentence/init", func(w http.ResponseWriter, r *http.Request) {
		db, err := sql.Open("mysql", getConnectionString(""))
		if err != nil {
			panic(err.Error())
		}
		defer db.Close()

		_, err = db.Exec("CREATE DATABASE Book")
		if err != nil {
			panic(err.Error())
		}

		_, err = db.Exec("USE Book")
		if err != nil {
			panic(err.Error())
		}

		createTable, err := db.Prepare("CREATE TABLE Sentences (Sentence VARCHAR(100))")
		if err != nil {
			panic(err.Error())
		}

		_, err = createTable.Exec()
		if err != nil {
			panic(err.Error())
		}

		fmt.Fprintf(w, "Database and table initialized on DEV")
	})

	// This endpoint adds a new record into the database
	http.HandleFunc("/sentence/add", func(w http.ResponseWriter, r *http.Request) {
		db, err := sql.Open("mysql", getConnectionString("Book"))
		if err != nil {
			panic(err.Error())
		}
		defer db.Close()

		_, err = db.Exec("INSERT INTO Sentences VALUES ('All work and no play makes Jack a dull boy')")
		if err != nil {
			panic(err.Error())
		}

		fmt.Fprintf(w, "New sentence added!")
	})

	// This endpoint just returns all of the available rows in the database
	http.HandleFunc("/sentence/get", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Current sentences in db:\n\n")

		db, err := sql.Open("mysql", getConnectionString("Book"))
		if err != nil {
			panic(err.Error())
		}
		defer db.Close()

		results, err := db.Query("SELECT Sentence FROM Sentences")
		if err != nil {
			panic(err.Error())
		}

		// For each returned row from the above query, put the row into the a Sentences struct
		// and print the row on the webpage
		for results.Next() {
			var row Sentences
			err = results.Scan(&row.Sentence)
			if err != nil {
				panic(err.Error())
			}

			fmt.Fprintf(w, "%s\n", row.Sentence)
		}
	})

	fmt.Println("running service. . .")
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		panic(err.Error())
	}
}

/* This method is responsible for creating the connection string
*  that is used for connecting to the database, regardless of
*  the environment that the application is deployed to (i.e. local or dev) */
func getConnectionString(dbname string) string {

	// This project mounts four environment variables onto the pod of the application
	// DB_USERNAME, DB_PASSWORD, DB_HOST, and DB_PORT

	// If these environment variables do not exist, we're not inside of Kubernetes
	// and we should just use some defaults as defined by the developer.
	username, env := os.LookupEnv("DB_USERNAME")
	if !env {
		username = "admin"
	}

	password, env := os.LookupEnv("DB_PASSWORD")
	if !env {
		password = "admin"
	}

	host, env := os.LookupEnv("DB_HOST")
	if !env {
		host = "127.0.0.1"
	}

	port, env := os.LookupEnv("DB_PORT")
	if !env {
		port = "3306"
	}

	connectionString := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", username, password, host, port, dbname)

	return connectionString
}
