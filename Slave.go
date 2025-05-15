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
	"runtime"
	"sync"
	"time"
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
	databases = make(map[string]*Database)
	dbMu      sync.Mutex
	dataFile  = "data.json"
	slaveFile = "slave_data.json"
    slavePort = "8001" // Default port
)

// ===================== INIT =====================

func initSlaveDatabase() {
	// Load slave data if available
	if _, err := os.Stat(slaveFile); err == nil {
		content, err := ioutil.ReadFile(slaveFile)
		if err == nil {
			json.Unmarshal(content, &databases)
			fmt.Println("Loaded slave data from slave_data.json")
			return
		}
	}
	fmt.Println("No existing data file found for slave.")
	databases = make(map[string]*Database)
}

func saveSlaveDataToFile() {
	content, _ := json.MarshalIndent(databases, "", "  ")
	ioutil.WriteFile(slaveFile, content, 0644)
}

func initDatabaseStorage() {
	// Load master data if available
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
	fmt.Println("Slave node starting on port 8001...") // Change port as needed
	initSlaveDatabase()
fs := http.FileServer(http.Dir("slave"))
	http.Handle("/", fs)
	http.HandleFunc("/replicate_insert", handleReplicateInsert)
	http.HandleFunc("/replicate_update", handleReplicateUpdate)
	http.HandleFunc("/replicate_delete", handleReplicateDelete)
	http.HandleFunc("/replicate_get", handleGetData)

	go func() {
		log.Fatal(http.ListenAndServe(":"+slavePort, nil))
	}()

	// Wait a moment for server to start
	time.Sleep(500 * time.Millisecond)

	// Open browser automatically
	openBrowser("http://localhost:" + slavePort)
	
	// Keep the program running
	select {}

}

// ===================== BROWSER UTILS =====================

func openBrowser(url string) {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // linux, freebsd, etc.
		cmd = "xdg-open"
		args = []string{url}
	}

	if err := exec.Command(cmd, args...).Start(); err != nil {
		log.Printf("Failed to open browser: %v", err)
	}
}

// ===================== REPLICATION =====================

func replicateInsert(req RequestData) {
	db, ok := databases[req.Database]
	if !ok {
		log.Println("Database not found")
		return
	}
	table, ok := db.Tables[req.Table]
	if !ok {
		log.Println("Table not found")
		return
	}

	// Lock table while inserting
	table.mu.Lock()
	defer table.mu.Unlock()
	table.Records = append(table.Records, req.Record)
	saveSlaveDataToFile()
	log.Println("Data inserted in slave")
}

func replicateUpdate(req RequestData) {
	db, ok := databases[req.Database]
	if !ok {
		log.Println("Database not found")
		return
	}
	table, ok := db.Tables[req.Table]
	if !ok {
		log.Println("Table not found")
		return
	}

	// Lock table while updating
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
	saveSlaveDataToFile()
	log.Printf("Updated %d records in slave", updated)
}

func replicateDelete(req RequestData) {
	db, ok := databases[req.Database]
	if !ok {
		log.Println("Database not found")
		return
	}
	table, ok := db.Tables[req.Table]
	if !ok {
		log.Println("Table not found")
		return
	}

	// Lock table while deleting
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
	saveSlaveDataToFile()
	log.Printf("Deleted %d records in slave", deleted)
}



// Function to replicate data from master to slave (Insert)
func replicateToSlaveInsert(req RequestData) {
    slaveURL := "http://localhost:8001/replicate_insert"  // عنوان السلاف
    jsonData, err := json.Marshal(req)
    if err != nil {
        log.Printf("Error marshalling data: %v", err)
        return
    }

    // إرسال البيانات إلى السلاف
    resp, err := http.Post(slaveURL, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        log.Printf("Error sending data to slave: %v", err)
        return
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == http.StatusOK {
        log.Println("Data successfully replicated to slave")
    } else {
        log.Printf("Failed to replicate data to slave. Status: %s", resp.Status)
    }
}

// Function to replicate data from master to slave (Update)
func replicateToSlaveUpdate(req RequestData) {
    slaveURL := "http://localhost:8001/replicate_update"  // عنوان السلاف
    jsonData, err := json.Marshal(req)
    if err != nil {
        log.Printf("Error marshalling data: %v", err)
        return
    }

    // إرسال البيانات إلى السلاف
    resp, err := http.Post(slaveURL, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        log.Printf("Error sending data to slave: %v", err)
        return
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == http.StatusOK {
        log.Println("Data successfully replicated to slave")
    } else {
        log.Printf("Failed to replicate data to slave. Status: %s", resp.Status)
    }
}

// Function to replicate data from master to slave (Delete)
func replicateToSlaveDelete(req RequestData) {
    slaveURL := "http://localhost:8001/replicate_delete"  // عنوان السلاف
    jsonData, err := json.Marshal(req)
    if err != nil {
        log.Printf("Error marshalling data: %v", err)
        return
    }

    // إرسال البيانات إلى السلاف
    resp, err := http.Post(slaveURL, "application/json", bytes.NewBuffer(jsonData))
    if err != nil {
        log.Printf("Error sending data to slave: %v", err)
        return
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == http.StatusOK {
        log.Println("Data successfully replicated to slave")
    } else {
        log.Printf("Failed to replicate data to slave. Status: %s", resp.Status)
    }
}

// ===================== HANDLERS =====================

// Handle incoming insert requests
func handleReplicateInsert(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
        return
    }
    var req RequestData
    json.NewDecoder(r.Body).Decode(&req)

    // إضافة السجل إلى قاعدة بيانات السلاف
    db, ok := databases[req.Database]
    if !ok {
        db = &Database{
            Name:   req.Database,
            Tables: make(map[string]*Table),
        }
        databases[req.Database] = db
    }

    table, ok := db.Tables[req.Table]
    if !ok {
        table = &Table{
            Name:    req.Table,
            Columns: req.Columns,
            Records: []map[string]string{},
        }
        db.Tables[req.Table] = table
    }

    // إضافة السجل
    table.mu.Lock()
    table.Records = append(table.Records, req.Record)
    table.mu.Unlock()

    // حفظ البيانات في السلاف
    saveSlaveDataToFile()

    w.Write([]byte("Record inserted successfully on slave"))
}

// Handle incoming update requests
func handleReplicateUpdate(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
        return
    }
    var req RequestData
    json.NewDecoder(r.Body).Decode(&req)

    // تحديث السجل في السلاف بناءً على الشروط
    db, ok := databases[req.Database]
    if !ok {
        db = &Database{
            Name:   req.Database,
            Tables: make(map[string]*Table),
        }
        databases[req.Database] = db
    }

    table, ok := db.Tables[req.Table]
    if !ok {
        table = &Table{
            Name:    req.Table,
            Columns: req.Columns,
            Records: []map[string]string{},
        }
        db.Tables[req.Table] = table
    }

    table.mu.Lock()
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
    table.mu.Unlock()

    saveSlaveDataToFile()

    w.Write([]byte(fmt.Sprintf("Updated %d records in slave.", updated)))
}

// Handle incoming delete requests
func handleReplicateDelete(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
        return
    }
    var req RequestData
    json.NewDecoder(r.Body).Decode(&req)

    // حذف السجل في السلاف بناءً على الشروط
    db, ok := databases[req.Database]
    if !ok {
        db = &Database{
            Name:   req.Database,
            Tables: make(map[string]*Table),
        }
        databases[req.Database] = db
    }

    table, ok := db.Tables[req.Table]
    if !ok {
        table = &Table{
            Name:    req.Table,
            Columns: req.Columns,
            Records: []map[string]string{},
        }
        db.Tables[req.Table] = table
    }

    table.mu.Lock()
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
    table.mu.Unlock()

    saveSlaveDataToFile()

    w.Write([]byte(fmt.Sprintf("Deleted %d records in slave.", deleted)))
}

// Handle displaying data in slave
func handleGetData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET allowed", http.StatusMethodNotAllowed)
		return
	}
	dbName := r.URL.Query().Get("database")
	tableName := r.URL.Query().Get("table")

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

	// Lock the table while reading the data
	table.mu.Lock()
	defer table.mu.Unlock()
	json.NewEncoder(w).Encode(table.Records)
}
