package radar

import (
	"net/url"
	"strings"
)

type Contact struct {
	Type string `json:"type"`
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
