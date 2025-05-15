
# Distributed JSON-based Master-Slave Database System (Go)

This project implements a distributed database system based on the Master-Slave Replication model. The system ensures data redundancy and availability through replication between a master node and a slave node. Additionally, we have developed a simple GUI to interact with the database system.
---

## 📌 Features

- Master-Slave architecture with replication
- Data persistence in JSON files (`data.json`, `slave_data.json`)
- Support for:
  - Create database/table
  - Insert, Update, Delete records
  - Automatic replication to slave


---


## 🧩 Technologies

- **Language**: Go (Golang)
- **Data Storage**: JSON files
- **Communication**: HTTP (via `net/http`)
- **Concurrency**: Goroutines & Channels

----

## 🏗️ Architecture

```
               +------------------+
               |   Master Node    |
               |------------------|
               | - DB Write Access|
               | - Broadcast to   |
               |   all Slaves     |
               +--------+---------+
                        |
      +-----------------+------------------+
      |                                    |
+-----v-----+                      +--------v------+
| Slave Node|                      |  Slave Node   |
|-----------|                      |---------------|
|- Full CRUD|                      | - Full CRUD   |
|           |                                         
+-----------+                      +---------------+
```





## 🧱 Project Structure

```text
.
├── master.go       # Main master server
├── slave.go        # Main slave server
├── data.json       # Master data file (auto-created)
├── slave_data.json # Slave data file (auto-created)
└── README.md
```

---

## 🔗 API Endpoints


### ✅ Master API (Port 8000)

| Method | Endpoint               | Description               |
|--------|------------------------|---------------------------|
| POST   | `/create_database`     | Create a new database     |
| DELETE | `/delete_database`     | Delete an existing database|
| GET    | `/list_databases`      | List all databases        |
| POST   | `/create_table`        | Create a new table        |
| POST   | `/insert`              | Insert a new record       |
| POST   | `/update`              | Update existing records   |
| POST   | `/delete`              | Delete records            |
| GET    | `/get_data`            | Get table data            |

### ✅ Slave API (Port 8001)

| Method | Endpoint             | Description               |
|--------|----------------------|---------------------------|
| POST   | `/replicate_insert`  | Insert replication        |
| POST   | `/replicate_update`  | Update replication        |
| POST   | `/replicate_delete`  | Delete replication        |
| GET    | `/replicate_get`     | Get replicated data       |


---



## 💡 Notes

- Replication to the slave is done asynchronously using `go` goroutines.
- Data is stored in local files (`data.json` for master, `slave_data.json` for slave).
- Tables are created dynamically on insert if they don’t exist.
- No external database dependency.

---


## 🚀 How to Run

### 1. Run the Slave Node

```bash
go run slave.go
```

This will start the slave server on `localhost:8001`.

### 2. Run the Master Node

```bash
go run master.go
```

This will start the master server on `localhost:8000`.

---








