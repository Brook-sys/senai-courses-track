package notifier

import (
	"strings"
	"testing"

	"github.com/Brook-sys/senai-courses-track/internal/scraper"
)

func TestFormatCourseMessage(t *testing.T) {
	course := scraper.Course{
		Title:    "Soldador ao Arco Elétrico e Oxigás",
		Duration: "168 horas",
		URL:      "https://www.sp.senai.br/curso/soldador-ao-arco-eletrico-e-oxigas/110125",
		Turmas: []scraper.Turma{
			{
				Unit:      "Piracicaba - Jardim Primavera",
				Address:   "Av. Marechal Castelo Branco, 1000",
				Phone:     "(19) 3412-3500",
				Vacancies: "16",
				StartDate: "20/07/2026",
				EndDate:   "08/10/2026",
				Schedule:  "13:15 às 17:15",
				Period:    "de 2ª, 3ª, 4ª e 5ª feira",
			},
		},
	}

	msg := formatCourseMessage(course, "Teste Hermes")

	expectedLines := []string{
		"🔔 Nova oportunidade em: *Teste Hermes*",
		"📚 *Soldador ao Arco Elétrico e Oxigás*",
		"⏱️ Carga horária: 168 horas",
		"🔗 Abrir curso (https://www.sp.senai.br/curso/soldador-ao-arco-eletrico-e-oxigas/110125)",
		"🏫 Unidade: Piracicaba - Jardim Primavera",
		"📍 Endereço: Av. Marechal Castelo Branco, 1000",
		"☎️ Telefone: (19) 3412-3500",
		"🎟️ Vagas: 16",
		"📅 Início/Fim: 20/07/2026 até 08/10/2026",
		"🗓️ Dias: de 2ª, 3ª, 4ª e 5ª feira",
		"🕒 Horário: 13:15 às 17:15",
	}

	for _, expected := range expectedLines {
		if !strings.Contains(msg, expected) {
			t.Errorf("Missing expected text: %s\\nGot:\\n%s", expected, msg)
		}
	}
}
