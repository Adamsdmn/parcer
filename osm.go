package radar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Источник данных: OpenStreetMap через Overpass API (бесплатно, без ключа).

var (
	overpassURLs = []string{
		"https://overpass-api.de/api/interpreter",
		"https://overpass.kumi.systems/api/interpreter",
		"https://overpass.private.coffee/api/interpreter",
	}
	osmClient = &http.Client{Timeout: 18 * time.Second}
	unsafeQL  = regexp.MustCompile(`[^\p{L}\p{N} \-]`)
)

type osmRule struct {
	stems   []string
	filters []string
}

// Порядок важен: более специфичные правила выше.
var osmRules = []osmRule{
	{[]string{"барбер"}, []string{`["shop"="hairdresser"]["name"~"барбер|barber",i]`}},
	{[]string{"ресторан"}, []string{`["amenity"="restaurant"]`}},
	{[]string{"кофейн", "кафе"}, []string{`["amenity"="cafe"]`}},
	{[]string{"пиццер"}, []string{`["cuisine"~"pizza"]`}},
	{[]string{"суши"}, []string{`["cuisine"~"sushi|japanese"]`}},
	{[]string{"фастфуд", "фаст-фуд"}, []string{`["amenity"="fast_food"]`}},
	{[]string{"пекарн"}, []string{`["shop"="bakery"]`}},
	{[]string{"парикмахер"}, []string{`["shop"="hairdresser"]`}},
	{[]string{"салон красот", "косметолог", "маникюр"}, []string{`["shop"="beauty"]`, `["shop"="hairdresser"]`}},
	{[]string{"массаж"}, []string{`["shop"="massage"]`}},
	{[]string{"стоматолог"}, []string{`["amenity"="dentist"]`}},
	{[]string{"ветеринар"}, []string{`["amenity"="veterinary"]`}},
	{[]string{"клиник"}, []string{`["amenity"~"clinic|doctors|hospital"]`}},
	{[]string{"аптек"}, []string{`["amenity"="pharmacy"]`}},
	{[]string{"компьютер", "клуб"}, []string{`["amenity"="internet_cafe"]`, `["leisure"="adult_gaming_centre"]`, `["name"~"cyber|кибер|компьютерный клуб|gaming club",i]`}},
	{[]string{"караоке"}, []string{`["amenity"="karaoke_box"]`, `["name"~"караоке|karaoke",i]`}},
	{[]string{"боулинг"}, []string{`["leisure"="bowling_alley"]`}},
	{[]string{"кинотеатр"}, []string{`["amenity"="cinema"]`}},
	{[]string{"кальян"}, []string{`["amenity"="hookah_lounge"]`, `["name"~"кальян|hookah",i]`}},
	{[]string{"квест"}, []string{`["name"~"квест|quest",i]`}},
	{[]string{"бильярд"}, []string{`["sport"="billiards"]`, `["name"~"бильярд",i]`}},
	{[]string{"фитнес", "спортзал"}, []string{`["leisure"="fitness_centre"]`}},
	{[]string{"бассейн"}, []string{`["leisure"="swimming_pool"]`, `["leisure"="sports_centre"]["sport"="swimming"]`}},
	{[]string{"автосервис"}, []string{`["shop"="car_repair"]`}},
	{[]string{"автомойк"}, []string{`["amenity"="car_wash"]`}},
	{[]string{"шиномонтаж"}, []string{`["shop"="tyres"]`}},
	{[]string{"автозапчаст"}, []string{`["shop"="car_parts"]`}},
	{[]string{"языков"}, []string{`["amenity"="language_school"]`}},
	{[]string{"детский сад"}, []string{`["amenity"="kindergarten"]`}},
	{[]string{"автошкол"}, []string{`["amenity"="driving_school"]`}},
	{[]string{"учебный центр"}, []string{`["amenity"="training"]`}},
	{[]string{"одежд"}, []string{`["shop"="clothes"]`}},
	{[]string{"цветы", "цветов"}, []string{`["shop"="florist"]`}},
	{[]string{"подарк"}, []string{`["shop"="gift"]`}},
	{[]string{"мебел"}, []string{`["shop"="furniture"]`}},
	{[]string{"электроник"}, []string{`["shop"="electronics"]`}},
	{[]string{"зоомагаз"}, []string{`["shop"="pet"]`}},
	{[]string{"фотостуд"}, []string{`["shop"="photo"]`, `["craft"="photographer"]`}},
	{[]string{"ремонт техник"}, []string{`["shop"="electronics_repair"]`, `["craft"="electronics_repair"]`}},
	{[]string{"типограф"}, []string{`["shop"="copyshop"]`, `["craft"="printer"]`}},
	{[]string{"юридич"}, []string{`["office"="lawyer"]`}},
	{[]string{"строител"}, []string{`["office"="construction_company"]`, `["craft"="builder"]`}},
	{[]string{"бар"}, []string{`["amenity"~"^(bar|pub)$"]`}},
}

func osmFilters(cat string) []string {
	q := strings.ToLower(cat)
	for _, r := range osmRules {
		for _, s := range r.stems {
			if strings.Contains(q, s) {
				return r.filters
			}
		}
	}
	// Незнакомая категория: ищем по названию среди любых коммерческих объектов.
	name := strings.TrimSpace(unsafeQL.ReplaceAllString(q, ""))
	if name == "" {
		return nil
	}
	return []string{`[~"^(amenity|shop|leisure|office|craft|tourism)$"~"."]["name"~"` + name + `",i]`}
}

func osmQuery(city string, filters []string) string {
	c := strings.TrimSpace(unsafeQL.ReplaceAllString(city, ""))
	var sb strings.Builder
	sb.WriteString("[out:json][timeout:20];\n")
	fmt.Fprintf(&sb, `(area["boundary"="administrative"]["name"="%s"];area["boundary"="administrative"]["name:ru"="%s"];)->.a;`+"\n(\n", c, c)
	for _, f := range filters {
		sb.WriteString("  nwr" + f + `["name"](area.a);` + "\n")
	}
	sb.WriteString(");\nout center tags 300;")
	return sb.String()
}

func overpass(q string) ([]byte, error) {
	var last error
	for _, ep := range overpassURLs {
		req, _ := http.NewRequest("POST", ep, strings.NewReader(url.Values{"data": {q}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", "RadarLeadFinder/1.0 (personal tool)")
		resp, err := osmClient.Do(req)
		if err != nil {
			last = err
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == 200 {
			return b, nil
		}
		last = fmt.Errorf("overpass %d", resp.StatusCode)
	}
	return nil, last
}

func tag(t map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(t[k]); v != "" {
			return strings.TrimSpace(strings.Split(v, ";")[0])
		}
	}
	return ""
}

func socialURL(base, v string) string {
	if strings.Contains(v, "/") || strings.Contains(v, ".") {
		return ensureHTTP(v)
	}
	return base + strings.TrimPrefix(v, "@")
}

func SearchOSM(city, cat string) ([]Place, error) {
	filters := osmFilters(cat)
	if len(filters) == 0 {
		return []Place{}, nil
	}
	b, err := overpass(osmQuery(city, filters))
	if err != nil {
		return nil, err
	}
	var raw struct {
		Elements []struct {
			Type   string  `json:"type"`
			ID     int64   `json:"id"`
			Lat    float64 `json:"lat"`
			Lon    float64 `json:"lon"`
			Center *struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			} `json:"center"`
			Tags map[string]string `json:"tags"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("overpass: неожиданный ответ")
	}

	places := []Place{}
	for _, e := range raw.Elements {
		t := e.Tags
		name := tag(t, "name:ru", "name")
		if name == "" {
			continue
		}
		lat, lon := e.Lat, e.Lon
		if e.Center != nil {
			lat, lon = e.Center.Lat, e.Center.Lon
		}
		addr := tag(t, "addr:full")
		if street := tag(t, "addr:street"); street != "" {
			addr = street
			if house := tag(t, "addr:housenumber"); house != "" {
				addr += ", " + house
			}
		}
		p := Place{
			ID: fmt.Sprintf("osm_%s_%d", e.Type, e.ID), Name: name, Category: capFirst(cat), City: city,
			Address: addr, Lat: lat, Lon: lon, Contacts: []Contact{},
			Link: fmt.Sprintf("https://www.openstreetmap.org/%s/%d", e.Type, e.ID),
		}

		hasSocial, hasTemplate, hasSite := false, false, false

		if v := tag(t, "phone", "contact:phone", "phone:mobile"); v != "" {
			d := digits(v)
			if len(d) == 11 && d[0] == '8' {
				d = "7" + d[1:]
			}
			p.Phone = v
			p.Contacts = append(p.Contacts, Contact{"phone", v, "tel:+" + d})
		}
		if v := tag(t, "website", "contact:website", "url"); v != "" {
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
		}
		socials := []struct {
			typ, base string
			keys      []string
		}{
			{"instagram", "https://instagram.com/", []string{"contact:instagram", "instagram"}},
			{"vkontakte", "https://vk.com/", []string{"contact:vk", "vk"}},
			{"telegram", "https://t.me/", []string{"contact:telegram", "telegram"}},
			{"facebook", "https://facebook.com/", []string{"contact:facebook", "facebook"}},
		}
		for _, s := range socials {
			if v := tag(t, s.keys...); v != "" {
				hasSocial = true
				p.Contacts = append(p.Contacts, Contact{s.typ, v, socialURL(s.base, v)})
			}
		}
		if v := tag(t, "contact:whatsapp", "whatsapp"); v != "" {
			hasSocial = true
			p.Contacts = append(p.Contacts, Contact{"whatsapp", v, "https://wa.me/" + digits(v)})
		}
		if v := tag(t, "contact:email", "email"); v != "" {
			p.Contacts = append(p.Contacts, Contact{"email", v, "mailto:" + v})
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
	return places, nil
}
