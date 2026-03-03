package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"chatterchat/internal/auth"
	"chatterchat/internal/db"
	"chatterchat/internal/models"
	"chatterchat/internal/ws"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// connEntry holds a WebSocket connection and its own write mutex.
type connEntry struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// ConnRegistry maps connection IDs to gorilla WebSocket connections.
type ConnRegistry struct {
	mu    sync.RWMutex
	conns map[string]*connEntry
}

func newConnRegistry() *ConnRegistry {
	return &ConnRegistry{conns: make(map[string]*connEntry)}
}

func (r *ConnRegistry) Add(connID string, conn *websocket.Conn) {
	r.mu.Lock()
	r.conns[connID] = &connEntry{conn: conn}
	r.mu.Unlock()
}

func (r *ConnRegistry) Remove(connID string) {
	r.mu.Lock()
	delete(r.conns, connID)
	r.mu.Unlock()
}

// Send writes data to connID. Returns nil if connID is not found
// (HandleDisconnect will clean up the DB record).
func (r *ConnRegistry) Send(_ context.Context, connID string, data []byte) error {
	r.mu.RLock()
	entry, ok := r.conns[connID]
	r.mu.RUnlock()
	if !ok {
		return nil
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	return entry.conn.WriteMessage(websocket.TextMessage, data)
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	ctx := context.Background()

	// Fail fast if DB is unreachable.
	if _, err := db.Get(ctx); err != nil {
		log.Fatalf("db init: %v", err)
	}

	registry := newConnRegistry()
	ws.LocalSender = registry.Send

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Logger)
	r.Use(corsMiddleware)

	// Public.
	r.Get("/health", healthHandler)

	// Authenticated routes (LOCAL_DEV_USER bypasses JWT validation).
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware)
		r.Get("/rooms", listRoomsHandler)
		r.Post("/rooms", createRoomHandler)
		r.Get("/rooms/{id}", getRoomHandler)
		r.Get("/rooms/{id}/messages", getMessagesHandler)
	})

	// WebSocket upgrade endpoint.
	r.Get("/ws", wsHandler(registry))

	log.Println("local server listening on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("ListenAndServe: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func wsHandler(registry *ConnRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade: %v", err)
			return
		}

		connID := "local-" + uuid.New().String()[:8]
		sub, username, email := devUser()

		ctx := context.Background()
		sqlDB, err := db.Get(ctx)
		if err != nil {
			log.Printf("ws db: %v", err)
			conn.Close()
			return
		}

		if err := ws.HandleConnect(ctx, sqlDB, connID, sub, username, email); err != nil {
			log.Printf("HandleConnect: %v", err)
			conn.Close()
			return
		}
		log.Printf("local WS connected: connID=%s user=%s", connID, username)
		registry.Add(connID, conn)

		defer func() {
			if err := ws.HandleDisconnect(ctx, sqlDB, "localhost", "local", connID); err != nil {
				log.Printf("HandleDisconnect: %v", err)
			}
			registry.Remove(connID)
			conn.Close()
		}()

		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				break
			}
			var msg ws.InboundMessage
			if err := json.Unmarshal(raw, &msg); err != nil {
				log.Printf("unmarshal ws message: %v", err)
				continue
			}
			switch msg.Action {
			case "joinRoom":
				if err := ws.HandleJoinRoom(ctx, sqlDB, "localhost", "local", connID, msg.RoomID); err != nil {
					log.Printf("HandleJoinRoom: %v", err)
				}
			case "sendMessage":
				if err := ws.HandleSendMessage(ctx, sqlDB, "localhost", "local", connID, msg.RoomID, msg.Body); err != nil {
					log.Printf("HandleSendMessage: %v", err)
				}
			case "ping":
				if err := registry.Send(ctx, connID, []byte(`{"type":"pong"}`)); err != nil {
					log.Printf("send pong: %v", err)
				}
			default:
				log.Printf("unknown WS action: %s", msg.Action)
			}
		}
	}
}

func devUser() (sub, username, email string) {
	raw := os.Getenv("LOCAL_DEV_USER")
	if raw == "" {
		return "dev-sub-001", "devuser", "dev@local.dev"
	}
	parts := strings.SplitN(raw, ":", 3)
	sub = parts[0]
	if len(parts) > 1 {
		username = parts[1]
	}
	if len(parts) > 2 {
		email = parts[2]
	}
	return
}

// ---- HTTP handlers (mirrors cmd/http-api/main.go) ----

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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
