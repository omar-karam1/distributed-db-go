package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
	"runtime"
)

// ===================== DATA STRUCTURES =====================

type Table struct {
	Name    string              `json:"name"`
	Columns []string            `json:"columns"`
	Records []map[string]string `json:"records"`
	mu      sync.Mutex          `json:"-"`
}

type Database struct {
	Name   string            `json:"name"`
	Tables map[string]*Table `json:"tables"`
}

type RequestData struct {
	Database   string            `json:"database"`
	Table      string            `json:"table"`
	Columns    []string          `json:"columns"`
	Record     map[string]string `json:"record"`
	UpdateData map[string]string `json:"update_data"`
	Conditions map[string]string `json:"conditions"`
}

var (
	databases  = make(map[string]*Database)
	dbMu       sync.Mutex
	dataFile   = "data.json"
	slaveNodes = []string{
		"http://localhost:8001/replicate_insert",
		"http://localhost:8002/replicate_insert",
	}
)

// ===================== INIT =====================

func initDatabaseStorage() {
	if _, err := os.Stat(dataFile); err == nil {
		content, err := ioutil.ReadFile(dataFile)
		if err == nil {
			json.Unmarshal(content, &databases)
			fmt.Println("Loaded data from", dataFile)
			return
		}
	}
	fmt.Println("No existing data file found.")
	databases = make(map[string]*Database)
}

func saveDataToFile() {
	content, _ := json.MarshalIndent(databases, "", "  ")
	ioutil.WriteFile(dataFile, content, 0644)
}

// ===================== MAIN =====================

func main() {
	fmt.Println("Master node starting on port 8000...")
	initDatabaseStorage()

	// Serve HTML static files
	fs := http.FileServer(http.Dir("master"))
	http.Handle("/", fs)

	// API endpoints
	http.HandleFunc("/create_database", handleCreateDatabase)
	http.HandleFunc("/create_table", handleCreateTable)
	http.HandleFunc("/insert", handleInsert)
	http.HandleFunc("/select", handleSelect)
	http.HandleFunc("/update", handleUpdate)
	http.HandleFunc("/delete", handleDelete)
	http.HandleFunc("/drop_table", handleDropTable)
	http.HandleFunc("/drop_database", handleDropDatabase)
	http.HandleFunc("/list_databases", handleListDatabases)
	http.HandleFunc("/list_tables", handleListTables)
	http.HandleFunc("/describe_table", handleDescribeTable)

	// Open browser automatically
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser("http://localhost:8000")
	}()

	log.Fatal(http.ListenAndServe(":8000", nil))
}

func openBrowser(url string) {
	var cmd string
	var args []string

	switch os := runtime.GOOS; os {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // linux and others
		cmd = "xdg-open"
		args = []string{url}
	}
	exec.Command(cmd, args...).Start()
}

// ===================== HANDLERS =====================

func handleCreateDatabase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RequestData
	json.NewDecoder(r.Body).Decode(&req)

	dbMu.Lock()
	defer dbMu.Unlock()
	if _, exists := databases[req.Database]; exists {
		http.Error(w, "Database already exists", http.StatusConflict)
		return
	}

	databases[req.Database] = &Database{
		Name:   req.Database,
		Tables: make(map[string]*Table),
	}
	saveDataToFile()
	w.Write([]byte("Database created successfully."))
}

func handleCreateTable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RequestData
	json.NewDecoder(r.Body).Decode(&req)

	db, ok := databases[req.Database]
	if !ok {
		http.Error(w, "Database not found", http.StatusNotFound)
		return
	}

	if _, exists := db.Tables[req.Table]; exists {
		http.Error(w, "Table already exists", http.StatusConflict)
		return
	}

	db.Tables[req.Table] = &Table{
		Name:    req.Table,
		Columns: req.Columns,
		Records: []map[string]string{},
	}
	saveDataToFile()
	w.Write([]byte("Table created successfully."))
}

func handleInsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RequestData
	json.NewDecoder(r.Body).Decode(&req)

	db, ok := databases[req.Database]
	if !ok {
		http.Error(w, "Database not found", http.StatusNotFound)
		return
	}
	table, ok := db.Tables[req.Table]
	if !ok {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

	table.mu.Lock()
	table.Records = append(table.Records, req.Record)
	table.mu.Unlock()

	saveDataToFile()
	go replicateToSlaves(req, "replicate_insert")
	w.Write([]byte("Record inserted successfully."))
}

func handleSelect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET allowed", http.StatusMethodNotAllowed)
		return
	}
	dbName := r.URL.Query().Get("database")
	tableName := r.URL.Query().Get("table")
	limit := r.URL.Query().Get("limit")

	db, ok := databases[dbName]
	if !ok {
		http.Error(w, "Database not found", http.StatusNotFound)
		return
	}
	table, ok := db.Tables[tableName]
	if !ok {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()
	
	records := table.Records
	if limit != "" {
		// Convert limit to integer
		limitNum := 0
		fmt.Sscanf(limit, "%d", &limitNum)
		if limitNum > 0 && limitNum < len(records) {
			records = records[:limitNum]
		}
	}
	
	json.NewEncoder(w).Encode(records)
}

func handleDescribeTable(w http.ResponseWriter, r *http.Request) {
	dbName := r.URL.Query().Get("database")
	tableName := r.URL.Query().Get("table")
	
	dbMu.Lock()
	defer dbMu.Unlock()
	
	db, ok := databases[dbName]
	if !ok {
		http.Error(w, "Database not found", http.StatusNotFound)
		return
	}
	
	table, ok := db.Tables[tableName]
	if !ok {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}
	
	response := struct {
		Columns []string `json:"columns"`
	}{
		Columns: table.Columns,
	}
	
	json.NewEncoder(w).Encode(response)
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RequestData
	json.NewDecoder(r.Body).Decode(&req)

	db, ok := databases[req.Database]
	if !ok {
		http.Error(w, "Database not found", http.StatusNotFound)
		return
	}
	table, ok := db.Tables[req.Table]
	if !ok {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()
	updated := 0
	for _, record := range table.Records {
		match := true
		for k, v := range req.Conditions {
			if record[k] != v {
				match = false
				break
			}
		}
		if match {
			for k, v := range req.UpdateData {
				record[k] = v
			}
			updated++
		}
	}
	saveDataToFile()
	go replicateUpdate(req)
	w.Write([]byte(fmt.Sprintf("Updated %d records.", updated)))
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RequestData
	json.NewDecoder(r.Body).Decode(&req)

	db, ok := databases[req.Database]
	if !ok {
		http.Error(w, "Database not found", http.StatusNotFound)
		return
	}
	table, ok := db.Tables[req.Table]
	if !ok {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

	table.mu.Lock()
	defer table.mu.Unlock()
	filtered := []map[string]string{}
	deleted := 0
	for _, record := range table.Records {
		match := true
		for k, v := range req.Conditions {
			if record[k] != v {
				match = false
				break
			}
		}
		if !match {
			filtered = append(filtered, record)
		} else {
			deleted++
		}
	}
	table.Records = filtered
	saveDataToFile()
	go replicateDelete(req)
	w.Write([]byte(fmt.Sprintf("Deleted %d records.", deleted)))
}

func handleDropTable(w http.ResponseWriter, r *http.Request) {
	var req RequestData
	json.NewDecoder(r.Body).Decode(&req)

	db, ok := databases[req.Database]
	if !ok {
		http.Error(w, "Database not found", http.StatusNotFound)
		return
	}

	delete(db.Tables, req.Table)
	saveDataToFile()
	w.Write([]byte(fmt.Sprintf("Table %s dropped from %s", req.Table, req.Database)))
}

func handleDropDatabase(w http.ResponseWriter, r *http.Request) {
	var req RequestData
	json.NewDecoder(r.Body).Decode(&req)

	delete(databases, req.Database)
	saveDataToFile()
	w.Write([]byte(fmt.Sprintf("Database %s dropped", req.Database)))
}

func handleListDatabases(w http.ResponseWriter, r *http.Request) {
	dbMu.Lock()
	defer dbMu.Unlock()

	dbNames := []string{}
	for name := range databases {
		dbNames = append(dbNames, name)
	}
	json.NewEncoder(w).Encode(dbNames)
}

func handleListTables(w http.ResponseWriter, r *http.Request) {
	dbName := r.URL.Query().Get("database")
	
	dbMu.Lock()
	defer dbMu.Unlock()
	
	db, ok := databases[dbName]
	if !ok {
		http.Error(w, "Database not found", http.StatusNotFound)
		return
	}
	
	tableNames := []string{}
	for name := range db.Tables {
		tableNames = append(tableNames, name)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tableNames)
}

// ===================== REPLICATION =====================

func replicateToSlaves(req RequestData, endpoint string) {
	for _, slave := range slaveNodes {
		go func(url string) {
			jsonData, _ := json.Marshal(req)
			http.Post(fmt.Sprintf("%s/%s", slave[:len(slave)-len("/replicate_insert")], endpoint), "application/json", bytes.NewBuffer(jsonData))
		}(slave)
	}
}

func replicateUpdate(req RequestData) {
	for _, slave := range slaveNodes {
		go func(url string) {
			jsonData, _ := json.Marshal(req)
			http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		}(slave)
	}
}

func replicateDelete(req RequestData) {
	for _, slave := range slaveNodes {
		go func(url string) {
			jsonData, _ := json.Marshal(req)
			http.Post(url, "application/json", bytes.NewBuffer(jsonData))
		}(slave)
	}
}