package scraper

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Turma struct {
	Unit         string
	Address      string
	Phone        string
	Modality     string
	Vacancies    string
	StartDate    string
	EndDate      string
	Period       string
	Schedule     string
	Registration string
	Observations string
	Requirements string
}

type Course struct {
	ID          string
	Title       string
	URL         string
	Modality    string
	City        string
	Free        bool
	StartDate   string
	Duration    string
	Description string
	HasTurmas   bool
	TurmaInfo   string
	Turmas      []Turma
}

type CityOption struct {
	Label string
	Value string
}

type Scraper struct {
	client *http.Client
	base   string
}

func New() *Scraper {
	return &Scraper{
		client: &http.Client{},
		base:   "https://www.sp.senai.br",
	}
}

func (s *Scraper) FetchCities() ([]CityOption, error) {
	resp, err := s.client.Get(s.base + "/cursos/0/0")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var cities []CityOption

	doc.Find("input[onchange*='GetQueryString(4']").Each(func(i int, input *goquery.Selection) {
		onchange, _ := input.Attr("onchange")
		value := extractBetween(onchange, "GetQueryString(4, 0, '", "')")
		if value == "" || seen[value] {
			return
		}
		label := compactText(input.Parent().Text())
		if label == "" {
			label = value
		}
		seen[value] = true
		cities = append(cities, CityOption{Label: label, Value: value})
	})

	return cities, nil
}

func (s *Scraper) FetchCourses(filters map[string]string) ([]Course, error) {
	u, _ := url.Parse(s.base + "/cursos/0/0")
	q := u.Query()
	for k, v := range filters {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	resp, err := s.client.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var courses []Course

	doc.Find(".card").Each(func(i int, card *goquery.Selection) {
		c := Course{}

		c.Title = strings.TrimSpace(card.Find(".card-title").First().Text())
		c.Description = strings.TrimSpace(card.Find(".card-text").First().Text())
		c.Duration = strings.TrimSpace(card.Find("p.horario strong").First().Text())

		card.Find(".badge").Each(func(i int, b *goquery.Selection) {
			txt := strings.TrimSpace(b.Text())
			if strings.Contains(strings.ToLower(txt), "distância") || strings.Contains(strings.ToLower(txt), "presencial") {
				c.Modality = txt
			}
		})

		c.City = strings.TrimSpace(card.Find(".verunidades li p strong").First().Text())

		// Parse javascript modal calls (openModalTurmas / openModalInfo)
		card.Find("*").Each(func(j int, el *goquery.Selection) {
			for _, attrName := range []string{"onclick", "href"} {
				val, ok := el.Attr(attrName)
				if !ok {
					continue
				}

				if strings.Contains(val, "openModalTurmas") {
					parts := parseJSArgs(val)
					if len(parts) < 9 {
						continue
					}

					c.HasTurmas = true
					c.TurmaInfo = val
					if c.Title == "" {
						c.Title = cleanHTML(parts[0])
					}

					slug := cleanHTML(parts[1])
					id := cleanHTML(parts[2])
					if c.ID == "" {
						c.ID = id
					}
					if slug != "" && id != "" {
						c.URL = fmt.Sprintf("%s/curso/%s/%s", s.base, slug, id)
					}

					// Extract the unit info from the same li as this button
					unitName := ""
					unitAddress := ""
					unitPhone := ""

					li := el.ParentsFiltered("li.list-group-item").First()
					if li.Length() > 0 {
						// O node li contem a unidade e o telefone. Para limparmos:
						liText := compactText(li.Find("p").First().Text())
						// Ex: "Piracicaba - Centro (19) 3437-4840"
						if idx := strings.Index(liText, "("); idx != -1 {
							unitName = strings.TrimSpace(liText[:idx])
							unitPhone = strings.TrimSpace(liText[idx:])
						} else {
							unitName = liText
						}

						// Address is in the fa-map-marker paragraph
						addrText := compactText(li.Find("p i.fa-map-marker").Parent().Text())
						if addrText != "" {
							unitAddress = addrText
						}
					}

					turmas, err := s.FetchTurmas(parts)
					if err == nil {
						// Overwrite or append the unit info from the listing
						for i := range turmas {
							if unitName != "" {
								turmas[i].Unit = unitName
							}
							if unitAddress != "" {
								turmas[i].Address = unitAddress
							}
							if unitPhone != "" {
								turmas[i].Phone = unitPhone
							}
						}
						c.Turmas = append(c.Turmas, turmas...)
					}
				}

				if strings.Contains(val, "openModalInfo") && c.ID == "" {
					parts := parseJSArgs(val)
					if len(parts) >= 2 {
						c.ID = cleanHTML(parts[len(parts)-1])
					}
				}
			}
		})

		// Fallback for clean URL if not found via modals
		if c.URL == "" {
			card.Find("a[href*='/curso/']").Each(func(j int, a *goquery.Selection) {
				h, _ := a.Attr("href")
				if strings.Contains(h, "facebook") || strings.Contains(h, "twitter") || strings.Contains(h, "linkedin") {
					return
				}
				if strings.HasPrefix(h, "/curso/") {
					c.URL = s.base + h
					parts := strings.Split(h, "/")
					if len(parts) > 0 {
						c.ID = parts[len(parts)-1]
					}
				}
			})
		}

		c.Free = strings.Contains(strings.ToLower(c.Title), "gratuito") || strings.Contains(strings.ToLower(c.Description), "gratuito")

		if c.Title != "" {
			courses = append(courses, c)
		}
	})

	return dedupeCourses(courses), nil
}

func dedupeCourses(courses []Course) []Course {
	byID := map[string]*Course{}
	var result []Course

	for _, c := range courses {
		key := c.ID
		if key == "" {
			key = c.URL + c.Title
		}

		existing, ok := byID[key]
		if !ok {
			copy := c
			copy.Turmas = dedupeTurmas(copy.Turmas)
			byID[key] = &copy
			result = append(result, copy)
			continue
		}

		existing.Turmas = dedupeTurmas(append(existing.Turmas, c.Turmas...))
		if existing.URL == "" {
			existing.URL = c.URL
		}
		if existing.City == "" {
			existing.City = c.City
		}
		for i := range result {
			if result[i].ID == key || (result[i].ID == "" && result[i].URL+result[i].Title == key) {
				result[i] = *existing
				break
			}
		}
	}

	return result
}

func dedupeTurmas(turmas []Turma) []Turma {
	seen := map[string]bool{}
	var result []Turma
	for _, t := range turmas {
		key := t.Unit + "|" + t.Address + "|" + t.StartDate + "|" + t.EndDate + "|" + t.Period + "|" + t.Schedule
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, t)
	}
	return result
}

func (s *Scraper) FetchTurmas(args []string) ([]Turma, error) {
	form := url.Values{}
	form.Set("nomeCurso", cleanHTML(args[1]))
	form.Set("cursoId", cleanHTML(args[2]))
	form.Set("escolaId", cleanHTML(args[3]))
	form.Set("estrategia", cleanHTML(args[4]))
	form.Set("bolsa", cleanHTML(args[6]))
	form.Set("gratuito", cleanHTML(args[6]))
	form.Set("turno", cleanHTML(args[7]))
	form.Set("pos", cleanHTML(args[8]))

	resp, err := s.client.PostForm(s.base+"/cursosturmas/", form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var turmas []Turma
	doc.Find(".card").Each(func(i int, card *goquery.Selection) {
		text := strings.TrimSpace(card.Text())
		if !strings.Contains(text, "Vagas:") || !strings.Contains(text, "Início") {
			return
		}

		t := Turma{}
		t.Unit = strings.TrimSpace(card.Find("p.card-text").First().Text())

		badges := card.Find(".badge")
		if badges.Length() > 0 {
			t.Modality = strings.TrimSpace(badges.Eq(0).Text())
		}
		badges.Each(func(i int, b *goquery.Selection) {
			v := strings.TrimSpace(b.Text())
			if strings.Contains(v, "Vagas:") {
				t.Vacancies = strings.TrimSpace(strings.TrimPrefix(v, "Vagas:"))
			}
			if strings.Contains(v, "INSCRIÇÃO") || strings.Contains(v, "BOLSA") {
				if t.Registration == "" {
					t.Registration = v
				} else {
					t.Registration += " | " + v
				}
			}
		})
		plain := compactText(text)
		t.StartDate = extractBetween(plain, "Início", "Fim")
		t.EndDate = extractBetween(plain, "Fim", "Período")

		// O Período e Horário estão estruturados em duas rows.
		// A primeira row tem os títulos, a segunda row tem os dados.
		card.Find(".row").Each(func(i int, row *goquery.Selection) {
			if strings.Contains(row.Text(), "Período") && strings.Contains(row.Text(), "Horário") {
				nextRow := row.NextFiltered(".row")
				if nextRow.Length() > 0 {
					cols := nextRow.Find(".col-6")
					if cols.Length() >= 2 {
						t.Period = strings.TrimSpace(cols.Eq(0).Text())
						t.Schedule = strings.TrimSpace(cols.Eq(1).Text())
					}
				}
			}
		})

		// Fallbacks just in case the HTML structure changes
		if t.Period == "" {
			t.Period = extractBetween(plain, "Período", "Horário")
		}
		if t.Schedule == "" {
			t.Schedule = extractBetween(plain, "Horário", "BOLSA")
		}
		if t.Schedule == "" {
			t.Schedule = extractBetween(plain, "Horário", "INSCRIÇÃO")
		}

		card.Find("[id^='collapseobs-']").Each(func(i int, s *goquery.Selection) {
			t.Observations = strings.TrimSpace(s.Text())
		})
		card.Find("[id^='collapsereq-']").Each(func(i int, s *goquery.Selection) {
			t.Requirements = strings.TrimSpace(s.Text())
		})

		turmas = append(turmas, t)
	})

	return turmas, nil
}

func parseJSArgs(jsCall string) []string {
	start := strings.Index(jsCall, "(")
	end := strings.LastIndex(jsCall, ")")
	if start == -1 || end == -1 {
		return nil
	}
	argsStr := jsCall[start+1 : end]

	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range argsStr {
		if !inQuote && (r == '\'' || r == '"') {
			inQuote = true
			quoteChar = r
			continue
		}
		if inQuote && r == quoteChar {
			inQuote = false
			args = append(args, current.String())
			current.Reset()
			continue
		}
		if !inQuote && r == ',' {
			if current.Len() > 0 {
				args = append(args, strings.TrimSpace(current.String()))
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		args = append(args, strings.TrimSpace(current.String()))
	}
	return args
}

func compactText(s string) string {
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(html.UnescapeString(re.ReplaceAllString(s, " ")))
}

func extractBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i == -1 {
		return ""
	}
	i += len(start)
	j := strings.Index(s[i:], end)
	if j == -1 {
		return strings.TrimSpace(s[i:])
	}
	return strings.TrimSpace(s[i : i+j])
}

func cleanHTML(s string) string {
	s = strings.ReplaceAll(s, "&#xC3;", "Ã")
	s = strings.ReplaceAll(s, "&#xE7;", "ç")
	s = strings.ReplaceAll(s, "&#xE3;", "ã")
	s = strings.ReplaceAll(s, "&#xE1;", "á")
	s = strings.ReplaceAll(s, "&#xE9;", "é")
	s = strings.ReplaceAll(s, "&#xED;", "í")
	s = strings.ReplaceAll(s, "&#xF3;", "ó")
	s = strings.ReplaceAll(s, "&#xFA;", "ú")
	return strings.Trim(s, " '\"")
}

func (s *Scraper) FetchCourseDetail(courseURL string) (*Course, error) {
	resp, err := s.client.Get(courseURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	c := &Course{URL: courseURL}
	c.Title = strings.TrimSpace(doc.Find("h1, .course-title").First().Text())
	c.Description = strings.TrimSpace(doc.Find(".description, .course-desc, p").First().Text())
	c.City = strings.TrimSpace(doc.Find(".city, .local").Text())
	c.Modality = strings.TrimSpace(doc.Find(".modality").Text())
	c.Duration = strings.TrimSpace(doc.Find(".duration, .carga").Text())
	c.StartDate = strings.TrimSpace(doc.Find(".start, .data-inicio").Text())

	return c, nil
}

func BuildFilters(city string, free, presencial bool) map[string]string {
	f := make(map[string]string)
	if city != "" {
		f["cidadeint"] = strings.ToLower(city)
	}
	if free {
		f["gratuito"] = "1"
	}
	if presencial {
		f["modalidade"] = "1"
	}
	return f
}

func (c Course) String() string {
	return fmt.Sprintf("%s | %s | %s | Free:%v | Turmas:%v | %s", c.Title, c.City, c.Modality, c.Free, c.HasTurmas, c.URL)
}
