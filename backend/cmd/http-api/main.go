package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"chatterchat/internal/auth"
	"chatterchat/internal/db"
	"chatterchat/internal/models"

	chiadapter "github.com/awslabs/aws-lambda-go-api-proxy/chi"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var chiLambda *chiadapter.ChiLambdaV2

func init() {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	// Public health check.
	r.Get("/health", healthHandler)

	// Authenticated routes — rely on API GW built-in JWT authorizer;
	// auth.Middleware used as belt-and-suspenders for local dev.
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware)
		r.Get("/rooms", listRoomsHandler)
		r.Post("/rooms", createRoomHandler)
		r.Get("/rooms/{id}", getRoomHandler)
		r.Get("/rooms/{id}/messages", getMessagesHandler)
	})

	chiLambda = chiadapter.NewV2(r)
}

func main() {
	lambda.Start(chiLambda.ProxyWithContext)
}

// ---- handlers ----

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func listRoomsHandler(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := db.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db unavailable")
		return
	}
	rooms, err := db.GetRooms(r.Context(), sqlDB)
	if err != nil {
		log.Printf("list rooms: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list rooms")
		return
	}
	writeJSON(w, http.StatusOK, rooms)
}

func createRoomHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	sqlDB, err := db.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db unavailable")
		return
	}
	room, err := db.CreateRoom(r.Context(), sqlDB, body.Name, body.Description)
	if err != nil {
		log.Printf("create room: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create room")
		return
	}
	writeJSON(w, http.StatusCreated, room)
}

func getRoomHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sqlDB, err := db.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db unavailable")
		return
	}
	room, err := db.GetRoomByID(r.Context(), sqlDB, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "room not found")
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func getMessagesHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	sqlDB, err := db.Get(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db unavailable")
		return
	}

	// Ensure user exists in DB — upsert on first read.
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		if _, err := db.UpsertUser(r.Context(), sqlDB, claims.Sub, claims.Username, claims.Email); err != nil {
			log.Printf("upsert user on message read: %v", err)
		}
	}

	msgs, err := db.GetMessagesByRoom(r.Context(), sqlDB, id, limit)
	if err != nil {
		log.Printf("get messages: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}
	// Return chronological order (DB returns newest-first).
	reversed := make([]models.Message, len(msgs))
	for i, m := range msgs {
		reversed[len(msgs)-1-i] = m
	}
	writeJSON(w, http.StatusOK, reversed)
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
