package ui

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/schmorrison/goshpanel/internal/terminal"
	"github.com/schmorrison/goshpanel/internal/ui/pages"
)

var terminalUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Terminal renders the web terminal page.
func (h *Handler) Terminal(w http.ResponseWriter, r *http.Request) {
	user, ok, err := h.requireUser(w, r)
	if !ok {
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !h.terminalEnabled {
		http.Error(w, "terminal disabled", http.StatusNotFound)
		return
	}

	pages.TerminalPage(user.Username, h.auth.CSRFToken(r.Context())).Render(r.Context(), w)
}

// TerminalWS provides a websocket-backed shell session.
func (h *Handler) TerminalWS(w http.ResponseWriter, r *http.Request) {
	if !h.terminalEnabled {
		http.Error(w, "terminal disabled", http.StatusNotFound)
		return
	}
	if _, ok, err := h.auth.CurrentUser(r.Context()); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	} else if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := terminalUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	session, err := terminal.NewSession(h.terminalShell, h.terminalWorkdir)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("failed to start terminal: "+err.Error()))
		return
	}
	defer session.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, err := session.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		default:
		}

		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}

		switch messageType {
		case websocket.TextMessage:
			var msg struct {
				Type string `json:"type"`
				Cols uint16 `json:"cols"`
				Rows uint16 `json:"rows"`
			}
			if json.Unmarshal(payload, &msg) == nil && msg.Type == "resize" {
				_ = session.Resize(msg.Cols, msg.Rows)
				continue
			}
			_, _ = session.Write(payload)
		case websocket.BinaryMessage:
			_, _ = session.Write(payload)
		}
	}
}
