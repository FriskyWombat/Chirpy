package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type SignedUser struct {
	ID           int    `json:"id"`
	Email        string `json:"email"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	IsPremium    bool   `json:"is_chirpy_red"`
}

type SafeUser struct {
	ID        int    `json:"id"`
	Email     string `json:"email"`
	IsPremium bool   `json:"is_chirpy_red"`
}

type User struct {
	ID                int    `json:"id"`
	Email             string `json:"email"`
	Password          string `json:"password"`
	RefreshToken      string `json:"refresh_token"`
	RefreshExpiration time.Time
	IsPremium         bool `json:"is_chirpy_red"`
}

func (d *Database) addUser(email string, password string) User {
	i := len(d.Users)
	d.Users[i+1] = User{
		ID:                i + 1,
		Email:             email,
		Password:          password,
		RefreshToken:      "",
		RefreshExpiration: time.Now().UTC(),
		IsPremium:         false,
	}
	return d.Users[i+1]
}

func (d *Database) getUserID(email string) int {
	for i, u := range d.Users {
		if u.Email == email {
			return i
		}
	}
	return -1
}

func (d *Database) getUserIDFromRefreshKey(key string) int {
	if key == "" {
		return -1
	}
	for i, u := range d.Users {
		if u.RefreshToken == key {
			return i
		}
	}
	return -1
}

// CreateExpiringJWTKey returns a string JWT key signed for the user
func createExpiringJWTKey(u User, expiresAfter time.Duration, token string) string {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expiresAfter)),
		Subject:   strconv.Itoa(u.ID),
	})
	str, err := t.SignedString([]byte(token))
	if err != nil {
		return err.Error()
	}
	return str
}

// CreateJWTKey returns a string JWT key signed for the user that expires after 24 hrs
func CreateJWTKey(u User, token string) string {
	return createExpiringJWTKey(u, time.Hour, token)
}

// ParseJWTKey returns a user associated with the valid JWT key if there is one
func ParseJWTKey(token string, secret string) (int, error) {
	t, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
	if err != nil {
		fmt.Println(err.Error())
		return 0, err
	}
	if claims, ok := t.Claims.(*jwt.RegisteredClaims); ok {
		id, err := strconv.Atoi(claims.Subject)
		if err != nil {
			return 0, err
		}
		return id, nil
	}
	return 0, fmt.Errorf("Authorization failed")
}

// CreateUser creates a new user and saves it to disk
func (db *DB) CreateUser(email string, password string) (User, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	data, err := db.loadDB()
	if err != nil {
		return User{}, err
	}
	i := data.getUserID(email)
	if i != -1 {
		return User{}, fmt.Errorf("A user with that email already exists")
	}
	pw, err := bcrypt.GenerateFromPassword([]byte(password), 0)
	u := data.addUser(email, string(pw))
	err = db.writeDB(data)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// UpdateUser changes the user with ID=id to the given email and password
func (db *DB) UpdateUser(id int, email string, password string) (User, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	data, err := db.loadDB()
	if err != nil {
		return User{}, err
	}
	u := data.Users[id]
	u.Email = email
	pw, err := bcrypt.GenerateFromPassword([]byte(password), 0)
	u.Password = string(pw)
	data.Users[id] = u
	err = db.writeDB(data)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// GetUser retrieves a User given an email
func (db *DB) GetUser(email string) (User, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
	data, err := db.loadDB()
	if err != nil {
		return User{}, err
	}
	userID := data.getUserID(email)
	if userID == -1 {
		return User{}, fmt.Errorf("Email does not exist")
	}
	return data.Users[userID], nil
}

// GetUserFromRefreshToken retrieves a User given a refresh token
func (db *DB) GetUserFromRefreshToken(token string) (User, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	data, err := db.loadDB()
	if err != nil {
		return User{}, err
	}
	i := data.getUserIDFromRefreshKey(token)
	if i == -1 {
		return User{}, fmt.Errorf("Token does not exist")
	}
	u := data.Users[i]
	if u.RefreshExpiration.Before(time.Now().UTC()) {
		return User{}, fmt.Errorf("Token expired")
	}
	return u, nil
}

// VerifyCredentials determines if the given password is valid for the given email
func (db *DB) VerifyCredentials(email string, password string) (User, bool, error) {
	u, err := db.GetUser(email)
	db.mux.RLock()
	defer db.mux.RUnlock()
	if err != nil {
		return User{}, false, err
	}
	err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	if err != nil {
		return User{}, false, nil
	}
	if u.RefreshToken == "" || u.RefreshExpiration.Before(time.Now().UTC()) {
		u, err = db.refreshUser(u.ID)
		if err != nil {
			return User{}, false, err
		}
	}
	return u, true, nil
}

// RefreshUser gives the user a new refresh token and resets the expiration date
func (db *DB) refreshUser(id int) (User, error) {
	data, err := db.loadDB()
	if err != nil {
		return User{}, err
	}
	u := data.Users[id]
	dat := make([]byte, 32)
	rand.Read(dat)
	u.RefreshToken = hex.EncodeToString(dat)
	u.RefreshExpiration = time.Now().UTC().Add(time.Hour * 24 * 60)
	data.Users[id] = u
	err = db.writeDB(data)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// RevokeRefreshToken deletes refresh token from a user with the given refresh token
func (db *DB) RevokeRefreshToken(token string) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	data, err := db.loadDB()
	if err != nil {
		return err
	}
	i := data.getUserIDFromRefreshKey(token)
	if i == -1 {
		return fmt.Errorf("Token does not exist")
	}
	u := data.Users[i]
	u.RefreshToken = ""
	u.RefreshExpiration = time.Now().UTC()
	data.Users[u.ID] = u
	err = db.writeDB(data)
	if err != nil {
		return err
	}
	return nil
}

// UpgradeUserToPremium activates Chirpy Red for the user with the given ID
func (db *DB) UpgradeUserToPremium(id int) error {
	db.mux.Lock()
	defer db.mux.Unlock()
	data, err := db.loadDB()
	if err != nil {
		return err
	}
	u := data.Users[id]
	u.IsPremium = true
	data.Users[id] = u
	err = db.writeDB(data)
	if err != nil {
		return err
	}
	return nil
}
