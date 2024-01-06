package main

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-telegram/bot/models"

	"marathon_procrastination_bot/internal/storage"
	"marathon_procrastination_bot/internal/telegram"
)

func Handler(w http.ResponseWriter, r *http.Request) {
	s, err := storage.New(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

	var (
		update        models.Update
		customRequest struct {
			WakeUp        bool `json:"wake_up,omitempty"`
			MigrateSchema bool `json:"migrate_schema,omitempty"`
		}
	)

	err = json.Unmarshal(body, &customRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch {
	case customRequest.WakeUp:
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
	case customRequest.MigrateSchema:
		if err := s.UpdateSchema(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		err = json.Unmarshal(body, &update)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, err = agent.Handle(r.Context(), agent.Bot(), &update)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}

}
