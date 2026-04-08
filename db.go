package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

var DB *sql.DB

// InitDB initializes the SQLite database and creates tables
func InitDB() {
	var err error
	// Open the database file (c2.db will be created automatically)
	DB, err = sql.Open("sqlite", "c2.db")
	if err != nil {
		log.Fatal("[-] Database Connection Error: ", err)
	}

	// Create Agents Table
	createTableSQL := `CREATE TABLE IF NOT EXISTS agents (
		id TEXT PRIMARY KEY,
		ip TEXT,
		hostname TEXT,
		platform TEXT,
		username TEXT,
		status TEXT,
		last_seen DATETIME
	);`

	_, err = DB.Exec(createTableSQL)
	if err != nil {
		log.Fatal("[-] Error Creating Table: ", err)
	}

	fmt.Println("[+] Database initialized (c2.db)")
}

// UpsertAgent inserts a new agent or updates an existing one
func UpsertAgent(agent *Agent) {
	// Use INSERT OR REPLACE logic for efficiency
	query := `INSERT INTO agents (id, ip, hostname, platform, username, status, last_seen) 
			  VALUES (?, ?, ?, ?, ?, ?, ?)
			  ON CONFLICT(id) DO UPDATE SET
			  ip=excluded.ip,
			  hostname=excluded.hostname,
			  username=excluded.username,
			  status=excluded.status,
			  last_seen=excluded.last_seen;`

	_, err := DB.Exec(query, agent.ID, agent.IP, agent.Hostname, agent.Platform, agent.Username, agent.Status, agent.LastSeen)
	if err != nil {
		log.Printf("[-] DB Error (UpsertAgent): %v\n", err)
	}
}

// LoadAgents loads all agents from the database into memory on startup
func LoadAgents() map[string]*Agent {
	loadedAgents := make(map[string]*Agent)

	rows, err := DB.Query("SELECT id, ip, hostname, platform, username, status, last_seen FROM agents")
	if err != nil {
		log.Printf("[-] DB Error (Query): %v\n", err)
		return loadedAgents
	}
	defer rows.Close()

	for rows.Next() {
		var a Agent
		var lastSeen time.Time
		err := rows.Scan(&a.ID, &a.IP, &a.Hostname, &a.Platform, &a.Username, &a.Status, &lastSeen)
		if err != nil {
			continue
		}
		a.LastSeen = lastSeen
		a.CommandQ = []string{} // Initialize empty queue
		loadedAgents[a.ID] = &a
	}

	return loadedAgents
}
