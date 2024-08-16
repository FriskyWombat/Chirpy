package database

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type Chirp struct {
	ID       int    `json:"id"`
	AuthorID int    `json:"author_id"`
	Body     string `json:"body"`
}

type Database struct {
	Users  map[int]User  `json:"users"`
	Chirps map[int]Chirp `json:"chirps"`
}

func (d *Database) addChirp(userID int, body string) Chirp {
	newID := 1
	for i := 1; ; i++ {
		_, ok := d.Chirps[i]
		if !ok {
			newID = i
			break
		}
	}
	d.Chirps[newID] = Chirp{newID, userID, body}
	return d.Chirps[newID]
}

type DB struct {
	path string
	mux  sync.RWMutex
}

// NewDB creates a new database connection
// and creates the database file if it doesn't exist
func NewDB(path string) (*DB, error) {
	db := DB{
		path: path,
	}
	err := db.ensureDB()
	return &db, err
}

// CreateChirp creates a new chirp and saves it to disk
func (db *DB) CreateChirp(id int, body string) (Chirp, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	data, err := db.loadDB()
	if err != nil {
		return Chirp{}, err
	}
	c := data.addChirp(id, body)
	err = db.writeDB(data)
	if err != nil {
		return Chirp{}, err
	}
	return c, nil
}

// GetChirp returns a single chirp with the given ID
func (db *DB) GetChirp(id int) (Chirp, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	data, err := db.loadDB()
	if err != nil {
		return Chirp{}, err
	}
	c, ok := data.Chirps[id]
	if !ok {
		return Chirp{}, fmt.Errorf("ID does not exist")
	}
	return c, nil
}

// GetChirps returns all chirps in the database
func (db *DB) GetChirps() ([]Chirp, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	data, err := db.loadDB()
	if err != nil {
		return []Chirp{}, err
	}
	chirps := make([]Chirp, 0, len(data.Chirps))
	for _, c := range data.Chirps {
		chirps = append(chirps, c)
	}
	return chirps, nil
}

// DeleteChirp deletes a single chirp with the given ID if it matches the user's JWT key
func (db *DB) DeleteChirp(id int, token string, secret string) error {
	c, err := db.GetChirp(id)
	if err != nil {
		return err
	}
	userID, err := ParseJWTKey(token, secret)
	if c.AuthorID == userID {
		db.forceDeleteChirp(id)
		return nil
	}
	return fmt.Errorf("Forbidden")
}

func (db *DB) forceDeleteChirp(id int) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	dat, err := os.ReadFile(db.path)
	if err != nil {
		return err
	}
	data := Database{make(map[int]User), make(map[int]Chirp)}
	err = json.Unmarshal(dat, &data)
	if err != nil {
		return err
	}
	delete(data.Chirps, id)
	dat, err = json.Marshal(&data)
	if err != nil {
		return err
	}
	err = os.WriteFile(db.path, dat, 0644)
	return err
}

// ensureDB creates a new database file if it doesn't exist
func (db *DB) ensureDB() error {
	db.mux.Lock()
	defer db.mux.Unlock()
	_, err := os.ReadFile(db.path)
	if os.IsNotExist(err) {
		d := Database{make(map[int]User), make(map[int]Chirp)}
		dat, _ := json.Marshal(&d)
		e2 := os.WriteFile(db.path, dat, 0644)
		if e2 != nil {
			return e2
		}
		return nil
	}
	return fmt.Errorf("file already exists")
}

// loadDB reads the database file into memory
func (db *DB) loadDB() (Database, error) {
	dat, err := os.ReadFile(db.path)
	if err != nil {
		return Database{}, err
	}
	data := Database{make(map[int]User), make(map[int]Chirp)}
	err = json.Unmarshal(dat, &data)
	if err != nil {
		return Database{}, err
	}
	return data, nil
}

// writeDB writes the database file to disk
func (db *DB) writeDB(d Database) error {
	dat, err := json.Marshal(&d)
	if err != nil {
		return err
	}
	err = os.WriteFile(db.path, dat, 0644)
	return err
}
