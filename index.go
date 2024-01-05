package main

import (
	"encoding/json"
	"fmt"
	"io"
	"marathon_procrastination_bot/internal/storage"
	"net/http"
	"time"

	"github.com/go-telegram/bot/models"

	"marathon_procrastination_bot/internal/telegram"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	s, err := storage.New(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Println(r.Header)

	if r.Header.Get("x-trigger") == "true" {
		ids, err := s.UsersForRotate(r.Context(),
			int32(time.Unix(int64(time.Now().UnixMilli()/1000/60/60)*60*60, 0).UTC().Hour()),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _, id := range ids {
			if err := s.RotateUserStats(r.Context(), id); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		return
	}

	agent, err := telegram.New(s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer func() {
		_ = r.Body.Close()
	}()

	update := &models.Update{}

	err = json.Unmarshal(body, update)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = agent.Handle(r.Context(), agent.Bot(), update)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
