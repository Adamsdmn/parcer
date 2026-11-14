package radar

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"unicode"
)

func h(s string) uint32 {
	f := fnv.New32a()
	f.Write([]byte(s))
	return f.Sum32()
}

func capFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

var demoNames = []struct {
	stem  string
	names []string
}{
	{"ресторан", []string{"Золотой Колос", "Aroma Steppe", "Печь & Дым", "Берег", "Nomad Grill", "Тенгри", "La Volta", "Саяхат", "Yurta House", "Ужин у Ани", "Casa Nova", "Иртыш"}},
	{"компьютер", []string{"CyberZone", "Respawn Arena", "GG Club", "HyperX Lounge", "Pixel Den", "Tilt", "Overclock", "Бункер", "Kill Switch", "Frag Point", "Лобби", "Ping"}},
	{"кафе", []string{"Кофейня №5", "Тёплый хлеб", "Brew Lab", "Чайхана Сары", "Булка", "Flat White", "Мята", "Сладкий сезон", "Утро", "Корица", "Эклер", "Зерно"}},
	{"караоке", []string{"Микрофон", "Голос", "Star Box", "Нота", "Бархат", "Хит", "Sing Room", "Мелодия", "Бит", "Октава", "Припев", "Дуэт"}},
	{"кальян", []string{"Дымка", "Облако", "Smoke Room", "Лаунж 21", "Аромат", "Туман", "Hookah Place", "Угли", "Эфир", "Пар", "Вайб", "Сизый"}},
	{"бар", []string{"Подвал", "Neon Tap", "Хмель", "Late Hours", "Барная стойка", "Ром", "Taproom", "Виски 12", "Крафт", "Шейкер", "Граница", "Лёд"}},
	{"боулинг", []string{"Страйк", "Кегли Плюс", "Strike Zone", "Шар", "Spare", "Лига", "Kingpin", "Дорожка", "Хэт-трик", "Двойной", "Кубок", "Финал"}},
	{"барбершоп", []string{"Борода", "Sharp", "Усы & Co", "Fade", "Бритва", "Cut Club", "Бобер", "Чёрный нож", "Тримм", "Стрижка 24", "Лезвие", "Мастер"}},
	{"салон", []string{"Лотос", "Beauty Room", "Орхидея", "Шарм", "Glow", "Пудра", "Муза", "Nail Bar", "Аура", "Лепесток", "Silk", "Грация"}},
}

var genericNames = []string{"Альфа", "Омега", "Премиум", "Стиль", "Лайт", "Центр", "Эксперт", "Прайм", "Мастер", "Сфера", "Профи", "Новатор"}

var demoStreets = []string{"ул. Ломова", "пр. Назарбаева", "ул. Академика Сатпаева", "ул. Торайгырова", "ул. Каирбаева", "ул. Естая", "ул. Луначарского", "ул. Толстого"}

func namesFor(cat string) []string {
	q := strings.ToLower(cat)
	for _, d := range demoNames {
		if strings.Contains(q, d.stem) {
			return d.names
		}
	}
	out := make([]string, len(genericNames))
	for i, g := range genericNames {
		out[i] = capFirst(cat) + " «" + g + "»"
	}
	return out
}

func DemoPlaces(city, cat string) []Place {
	out := []Place{}
	for i, n := range namesFor(cat) {
		seed := h(city + cat + n)
		rating := 3.4 + float64(seed%17)/10
		if rating > 5 {
			rating = 4.9
		}
		d := fmt.Sprintf("%09d", seed%1000000000)
		phone := "+7 7" + d[0:2] + " " + d[2:5] + "-" + d[5:7] + "-" + d[7:9]
		cs := []Contact{{"phone", phone, "tel:+77" + d}}

		status := "nosite"
		switch roll := seed % 10; {
		case roll <= 3:
			status = "nosite"
			if seed%2 == 0 {
				cs = append(cs, Contact{"whatsapp", "WhatsApp", "https://wa.me/77" + d})
			}
		case roll <= 6:
			status = "socials"
			cs = append(cs, Contact{"instagram", "instagram.com", "https://instagram.com/"}, Contact{"telegram", "Telegram", "https://t.me/"}, Contact{"whatsapp", "WhatsApp", "https://wa.me/77" + d})
		case roll == 7:
			status = "template"
			cs = append(cs, Contact{"website", "demo.tilda.ws", "https://demo.tilda.ws"}, Contact{"instagram", "instagram.com", "https://instagram.com/"})
		default:
			status = "site"
			cs = append(cs, Contact{"website", "example.kz", "https://example.kz"}, Contact{"instagram", "instagram.com", "https://instagram.com/"})
		}

		p := Place{
			ID: strconv.Itoa(int(seed)), Name: n, Category: capFirst(cat), City: city,
			Address:     demoStreets[int(seed)%len(demoStreets)] + ", " + strconv.Itoa(int(seed%90)+1),
			Rating:      float64(int(rating*10)) / 10,
			ReviewCount: int(seed%480) + 3,
			Status:      status, Phone: phone, Contacts: cs,
			Lat: 52.28 + float64(i)*0.003, Lon: 76.96 + float64(i)*0.004,
			Link: "https://2gis.kz/",
		}
		for _, c := range cs {
			if c.Type == "website" {
				p.Site = c.URL
			}
		}
		out = append(out, p)
	}
	return out
}

var demoTexts = []string{
	"Были с друзьями в пятницу вечером — заняли столик без брони, обслужили быстро. Вернёмся ещё.",
	"Очень уютно, приятная музыка не мешает разговору. Цены адекватные, порции большие.",
	"Хорошее место, но по выходным шумновато. Персонал вежливый, администратор подсказал, что заказать.",
	"Отмечали день рождения, всё прошло отлично. Отдельное спасибо за внимание к мелочам.",
	"Долго искали, где у них прайс и как записаться — сайта нет, пришлось звонить. В остальном всё понравилось.",
	"Чисто, светло, парковка рядом. Для семьи с ребёнком подходит.",
	"Немного дороговато, зато качество стабильное. Ходим сюда уже второй год.",
}
var demoAuthors = []string{"Алия", "Данияр", "Марина", "Тимур", "Ольга", "Ержан", "Сергей", "Айгерим"}

func DemoReviews(id string) []Review {
	seed := h(id)
	out := []Review{}
	for i := 0; i < 5; i++ {
		s := seed + uint32(i*31)
		out = append(out, Review{
			Author: demoAuthors[int(s)%len(demoAuthors)],
			Rating: 3 + int(s%3),
			Text:   demoTexts[int(s/3)%len(demoTexts)],
			Date:   "2026-" + pad(int(s%11)+1) + "-" + pad(int(s%27)+1),
		})
	}
	return out
}

func pad(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
