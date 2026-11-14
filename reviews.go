package handler

import (
	"encoding/json"
	"net/http"

	"gorod/internal/radar"
)

// Reviews — в OpenStreetMap отзывов нет, настоящие приходят только в демо-режиме.
func Reviews(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	id := r.URL.Query().Get("id")
	reviews := []radar.Review{}
	if len(id) > 0 && id[0] != 'o' { // id демо-данных — числа, id из OSM начинаются с "osm_"
		reviews = radar.DemoReviews(id)
	}
	json.NewEncoder(w).Encode(map[string]any{"reviews": reviews, "note": "В OpenStreetMap нет отзывов."})
}
