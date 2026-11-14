package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"gorod/internal/radar"
)

// Search — GET /api/search?city=Павлодар&cat=ресторан[&demo=1]
// Один запрос = одна пара город × категория.
func Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	city := strings.TrimSpace(q.Get("city"))
	cat := strings.TrimSpace(q.Get("cat"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if city == "" || cat == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Укажите город и категорию"})
		return
	}

	source := "osm"
	var places []radar.Place
	if q.Get("demo") != "" {
		source = "demo"
		places = radar.DemoPlaces(city, cat)
	} else {
		var err error
		places, err = radar.SearchOSM(city, cat)
		if err != nil {
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]string{"error": "Overpass (OpenStreetMap) сейчас перегружен или недоступен. Повторите через минуту."})
			return
		}
		// одинаковые запросы Vercel отдаёт из кэша — быстрее и не нагружает Overpass
		w.Header().Set("Cache-Control", "public, s-maxage=3600, stale-while-revalidate=86400")
	}
	json.NewEncoder(w).Encode(map[string]any{"demo": source == "demo", "source": source, "places": places})
}
