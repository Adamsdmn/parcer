package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

//go:embed static
var staticFS embed.FS

type Contact struct {
	Type string `json:"type"` // phone, whatsapp, telegram, instagram, vkontakte, viber, facebook, email, website, link
	Text string `json:"text"`
	URL  string `json:"url"`
}

// Place — одно заведение (потенциальный клиент).
type Place struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	City        string    `json:"city"`
	Address     string    `json:"address"`
	Rating      float64   `json:"rating"`
	ReviewCount int       `json:"reviewCount"`
	Status      string    `json:"status"` // nosite | socials | template | site
	Phone       string    `json:"phone,omitempty"`
	Site        string    `json:"site,omitempty"`
	Contacts    []Contact `json:"contacts"`
	Lat         float64   `json:"lat"`
	Lon         float64   `json:"lon"`
	Link        string    `json:"link"`
}

type Review struct {
	Author string `json:"author"`
	Rating int    `json:"rating"`
	Text   string `json:"text"`
	Date   string `json:"date"`
}

var (
	apiKey = os.Getenv("DGIS_KEY")
	source = detectSource() // osm (по умолчанию) | 2gis (если есть DGIS_KEY) | demo (SOURCE=demo)
	client = &http.Client{Timeout: 12 * time.Second}

	cacheMu sync.Mutex
	cache   = map[string]cacheItem{}
)

func detectSource() string {
	if s := os.Getenv("SOURCE"); s == "osm" || s == "2gis" || s == "demo" {
		return s
	}
	if apiKey != "" {
		return "2gis"
	}
	return "osm"
}

const (
	pageSize = 20
	maxPages = 2
	maxJobs  = 12
)

type cacheItem struct {
	data []byte
	exp  time.Time
}

func cached(key string, ttl time.Duration, fetch func() ([]byte, error)) ([]byte, error) {
	cacheMu.Lock()
	if it, ok := cache[key]; ok && time.Now().Before(it.exp) {
		cacheMu.Unlock()
		return it.data, nil
	}
	cacheMu.Unlock()
	b, err := fetch()
	if err != nil {
		return nil, err
	}
	cacheMu.Lock()
	cache[key] = cacheItem{b, time.Now().Add(ttl)}
	cacheMu.Unlock()
	return b, nil
}

func getJSON(u string, out any) error {
	resp, err := client.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("2GIS %d: %s", resp.StatusCode, b)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ---------- классификация сайта ----------

var socialHosts = []string{"instagram.com", "vk.com", "vk.ru", "facebook.com", "fb.com", "t.me", "telegram.me",
	"wa.me", "whatsapp.com", "taplink.", "linktr.ee", "tiktok.com", "youtube.com", "youtu.be", "ok.ru", "twitter.com", "viber.com"}

var builderHosts = []string{"tilda.ws", "tilda.cc", "wixsite.com", "wix.com", "weebly.com", "jimdosite.com", "nethouse.",
	"ucoz.", "business.site", "site123.me", "carrd.co", "mystrikingly.com", "blogspot.com", "wordpress.com", "flazio.com", "umi.ru", "2gis."}

func hostOf(raw string) string {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
}

func hostIn(h string, list []string) bool {
	for _, s := range list {
		if strings.Contains(h, s) {
			return true
		}
	}
	return false
}

func socialType(h string) string {
	switch {
	case strings.Contains(h, "instagram"):
		return "instagram"
	case strings.Contains(h, "vk.com"), strings.Contains(h, "vk.ru"):
		return "vkontakte"
	case strings.Contains(h, "t.me"), strings.Contains(h, "telegram"):
		return "telegram"
	case strings.Contains(h, "wa.me"), strings.Contains(h, "whatsapp"):
		return "whatsapp"
	case strings.Contains(h, "facebook"), strings.Contains(h, "fb.com"):
		return "facebook"
	case strings.Contains(h, "viber"):
		return "viber"
	}
	return "link"
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func ensureHTTP(v string) string {
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return v
	}
	return "https://" + v
}

// ---------- 2GIS Catalog API ----------

type dgisItems struct {
	Result struct {
		Items []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			AddressName string `json:"address_name"`
			Point       struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			} `json:"point"`
			Reviews struct {
				Rating float64 `json:"general_rating"`
				Count  int     `json:"general_review_count"`
			} `json:"reviews"`
			Rubrics []struct {
				Name string `json:"name"`
			} `json:"rubrics"`
			ContactGroups []struct {
				Contacts []struct {
					Type  string `json:"type"`
					Value string `json:"value"`
					Text  string `json:"text"`
				} `json:"contacts"`
			} `json:"contact_groups"`
		} `json:"items"`
	} `json:"result"`
}

func searchReal(city, cat string) ([]Place, error) {
	places := []Place{}
	for page := 1; page <= maxPages; page++ {
		u := "https://catalog.api.2gis.com/3.0/items?" + url.Values{
			"q":         {cat + " " + city},
			"key":       {apiKey},
			"page":      {fmt.Sprint(page)},
			"page_size": {fmt.Sprint(pageSize)},
			"locale":    {"ru_KZ"},
			"fields":    {"items.point,items.reviews,items.rubrics,items.contact_groups"},
		}.Encode()

		var raw dgisItems
		if err := getJSON(u, &raw); err != nil {
			if page == 1 {
				return nil, err
			}
			break
		}
		for _, it := range raw.Result.Items {
			p := Place{
				ID: it.ID, Name: it.Name, City: city, Address: it.AddressName,
				Rating: it.Reviews.Rating, ReviewCount: it.Reviews.Count,
				Lat: it.Point.Lat, Lon: it.Point.Lon, Contacts: []Contact{},
				Link: "https://2gis.kz/firm/" + strings.Split(it.ID, "_")[0],
			}
			if len(it.Rubrics) > 0 {
				p.Category = it.Rubrics[0].Name
			}
			hasSocial, hasTemplate, hasSite := false, false, false
			for _, g := range it.ContactGroups {
				for _, c := range g.Contacts {
					v := strings.TrimSpace(c.Value)
					if v == "" {
						continue
					}
					switch c.Type {
					case "phone":
						if p.Phone == "" {
							p.Phone = c.Text
						}
						p.Contacts = append(p.Contacts, Contact{"phone", c.Text, "tel:" + "+" + strings.TrimPrefix(digits(v), "+")})
					case "email":
						p.Contacts = append(p.Contacts, Contact{"email", v, "mailto:" + v})
					case "website":
						h := hostOf(v)
						switch {
						case hostIn(h, socialHosts):
							hasSocial = true
							p.Contacts = append(p.Contacts, Contact{socialType(h), h, ensureHTTP(v)})
						case hostIn(h, builderHosts):
							hasTemplate = true
							p.Site = ensureHTTP(v)
							p.Contacts = append(p.Contacts, Contact{"website", h, p.Site})
						default:
							hasSite = true
							p.Site = ensureHTTP(v)
							p.Contacts = append(p.Contacts, Contact{"website", h, p.Site})
						}
					case "whatsapp":
						hasSocial = true
						link := v
						if !strings.HasPrefix(v, "http") {
							link = "https://wa.me/" + digits(v)
						}
						p.Contacts = append(p.Contacts, Contact{"whatsapp", c.Text, link})
					case "telegram":
						hasSocial = true
						link := v
						if !strings.HasPrefix(v, "http") {
							link = "https://t.me/" + strings.TrimPrefix(v, "@")
						}
						p.Contacts = append(p.Contacts, Contact{"telegram", c.Text, link})
					case "viber":
						hasSocial = true
						p.Contacts = append(p.Contacts, Contact{"viber", c.Text, "viber://chat?number=%2B" + digits(v)})
					case "instagram", "vkontakte", "facebook", "youtube", "odnoklassniki", "twitter", "tiktok":
						hasSocial = true
						p.Contacts = append(p.Contacts, Contact{c.Type, v, ensureHTTP(v)})
					}
				}
			}
			switch {
			case hasSite:
				p.Status = "site"
			case hasTemplate:
				p.Status = "template"
			case hasSocial:
				p.Status = "socials"
			default:
				p.Status = "nosite"
			}
			places = append(places, p)
		}
		if len(raw.Result.Items) < pageSize {
			break
		}
	}
	return places, nil
}

func reviewsReal(id string) ([]Review, error) {
	id = strings.Split(id, "_")[0]
	u := fmt.Sprintf("https://public-api.reviews.2gis.com/3.0/branches/%s/reviews?limit=8&key=%s", url.PathEscape(id), apiKey)
	var raw struct {
		Reviews []struct {
			Text    string `json:"text"`
			Rating  int    `json:"rating"`
			Created string `json:"date_created"`
			User    struct {
				Name string `json:"name"`
			} `json:"user"`
		} `json:"reviews"`
	}
	if err := getJSON(u, &raw); err != nil {
		return nil, err
	}
	out := []Review{}
	for _, r := range raw.Reviews {
		if strings.TrimSpace(r.Text) == "" {
			continue
		}
		d := r.Created
		if len(d) >= 10 {
			d = d[:10]
		}
		out = append(out, Review{Author: r.User.Name, Rating: r.Rating, Text: r.Text, Date: d})
	}
	return out, nil
}

// ---------- handlers ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, "|") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func handleSearch(w http.ResponseWriter, r *http.Request) {
	cities := splitList(r.URL.Query().Get("cities"))
	cats := splitList(r.URL.Query().Get("cats"))
	if len(cities) == 0 || len(cats) == 0 {
		writeJSON(w, 400, map[string]string{"error": "Добавьте хотя бы один город и одну категорию"})
		return
	}
	if len(cities)*len(cats) > maxJobs {
		writeJSON(w, 400, map[string]string{"error": fmt.Sprintf("Слишком много комбинаций город × категория (максимум %d)", maxJobs)})
		return
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		sem   = make(chan struct{}, 4)
		seen  = map[string]bool{}
		all   = []Place{}
		fails = 0
		total = len(cities) * len(cats)
	)
	for _, city := range cities {
		for _, cat := range cats {
			wg.Add(1)
			go func(city, cat string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				var ps []Place
				var err error
				if source == "demo" {
					ps = demoPlaces(city, cat)
				} else {
					var b []byte
					b, err = cached("s:"+source+":"+city+"|"+cat, 30*time.Minute, func() ([]byte, error) {
						var p []Place
						var e error
						if source == "osm" {
							p, e = searchOSM(city, cat)
						} else {
							p, e = searchReal(city, cat)
						}
						if e != nil {
							return nil, e
						}
						return json.Marshal(p)
					})
					if err == nil {
						err = json.Unmarshal(b, &ps)
					}
				}
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					log.Println("search:", city, cat, err)
					fails++
					return
				}
				for _, p := range ps {
					if !seen[p.ID] {
						seen[p.ID] = true
						all = append(all, p)
					}
				}
			}(city, cat)
		}
	}
	wg.Wait()

	if fails == total {
		msg := "Источник данных не ответил. Проверьте интернет и попробуйте ещё раз."
		if source == "2gis" {
			msg = "2GIS не ответил. Проверьте ключ DGIS_KEY и интернет."
		} else if source == "osm" {
			msg = "Overpass (OpenStreetMap) сейчас перегружен или недоступен. Подождите минуту и повторите."
		}
		writeJSON(w, 502, map[string]string{"error": msg})
		return
	}
	writeJSON(w, 200, map[string]any{"demo": source == "demo", "source": source, "places": all, "failed": fails})
}

func handleReviews(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, 400, map[string]string{"error": "нет id"})
		return
	}
	if source == "demo" {
		writeJSON(w, 200, map[string]any{"reviews": demoReviews(id)})
		return
	}
	if source != "2gis" {
		writeJSON(w, 200, map[string]any{"reviews": []Review{}, "note": "В OpenStreetMap нет отзывов."})
		return
	}
	b, err := cached("r:"+id, 30*time.Minute, func() ([]byte, error) {
		rv, err := reviewsReal(id)
		if err != nil {
			return nil, err
		}
		return json.Marshal(rv)
	})
	if err != nil {
		log.Println("reviews:", err)
		writeJSON(w, 200, map[string]any{"reviews": []Review{}, "note": "2ГИС не отдаёт тексты отзывов через официальное API. Рейтинг и число отзывов выше — настоящие, а сами отзывы смотрите по кнопке «Открыть в 2ГИС»."})
		return
	}
	var rv []Review
	json.Unmarshal(b, &rv)
	writeJSON(w, 200, map[string]any{"reviews": rv})
}

func main() {
	sub, _ := fs.Sub(staticFS, "static")
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/search", handleSearch)
	mux.HandleFunc("/api/reviews", handleReviews)

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Println("источник данных:", source)
	log.Println("http://localhost" + addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
