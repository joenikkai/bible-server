package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
)

type User struct {
	UserName string `json:"user_name"`
	Age      uint8  `json:"age"`
	Gender   string `json:"gender"`
}

var userCache = make(map[int]User)

var cacheMutex sync.RWMutex

func main() {
	fmt.Println("Starting server!")
	mux := http.NewServeMux()

	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("POST /users", handleCreateUser)
	mux.HandleFunc("GET /users/{id}", handleReadUser)
	mux.HandleFunc("DELETE /users/{id}", handleRemoveUser)
	port := "0.0.0.0:9000"
	fmt.Println("Server listening on: ", port)
	http.ListenAndServe(port, mux)
}

func handleRoot(
	w http.ResponseWriter,
	r *http.Request,
) {
	fmt.Printf("REQUEST\n")
	fmt.Fprintf(w, "Welcome, User.\n")

}

func handleCreateUser(
	w http.ResponseWriter,
	r *http.Request,
) {
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusBadRequest,
		)
		return
	}
	if user.UserName == "" {
		http.Error(
			w,
			"field `user_name` was not prvided",
			http.StatusBadRequest,
		)
		return
	}
	if user.Gender == "" {
		http.Error(
			w,
			"field `gender` was not provided",
			http.StatusBadRequest,
		)
		return
	}

	cacheMutex.Lock()
	userCache[len(userCache)+1] = user
	cacheMutex.Unlock()
	w.WriteHeader(http.StatusNoContent)

}

func handleReadUser(
	w http.ResponseWriter,
	r *http.Request,
) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusBadRequest,
		)
		return
	}

	cacheMutex.RLock()
	user, ok := userCache[id]
	cacheMutex.RUnlock()
	if !ok {
		http.Error(
			w,
			"user not found",
			http.StatusNotFound,
		)
		return
	}
	user_j, err := json.Marshal(user)
	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusInternalServerError,
		)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(user_j)
	return
}

func handleRemoveUser(
	w http.ResponseWriter,
	r *http.Request,
) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(
			w,
			err.Error(),
			http.StatusBadRequest,
		)
		return
	}
	if _, ok := userCache[id]; !ok {
		http.Error(
			w,
			"user not found",
			http.StatusNotFound,
		)

	}
	cacheMutex.Lock()
	delete(userCache, id)
	cacheMutex.Unlock()
	w.WriteHeader(http.StatusNoContent)

}
