package notifier

import (
	"fmt"
	"strings"

	"github.com/Brook-sys/senai-courses-track/internal/scraper"
)

func formatCourseMessage(c scraper.Course, subName string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("🔔 Nova oportunidade em: *%s*\\n\\n", subName))
	sb.WriteString(fmt.Sprintf("📚 *%s*\\n", c.Title))

	if c.Duration != "" {
		sb.WriteString(fmt.Sprintf("⏱️ Carga horária: %s\\n", c.Duration))
	}
	sb.WriteString(fmt.Sprintf("🔗 Abrir curso (%s)\\n", c.URL))

	for _, turma := range c.Turmas {
		sb.WriteString("\\n")
		if turma.Unit != "" {
			sb.WriteString(fmt.Sprintf("🏫 Unidade: %s\\n", turma.Unit))
		}
		if turma.Address != "" {
			sb.WriteString(fmt.Sprintf("📍 Endereço: %s\\n", turma.Address))
		}
		if turma.Phone != "" {
			sb.WriteString(fmt.Sprintf("☎️ Telefone: %s\\n", turma.Phone))
		}
		if turma.Vacancies != "" {
			sb.WriteString(fmt.Sprintf("🎟️ Vagas: %s\\n", turma.Vacancies))
		}
		if turma.StartDate != "" || turma.EndDate != "" {
			sb.WriteString(fmt.Sprintf("📅 Início/Fim: %s até %s\\n", turma.StartDate, turma.EndDate))
		}
		if turma.Period != "" {
			sb.WriteString(fmt.Sprintf("🗓️ Dias: %s\\n", turma.Period))
		}
		if turma.Schedule != "" {
			sb.WriteString(fmt.Sprintf("🕒 Horário: %s\\n", turma.Schedule))
		}
	}

	return sb.String()
}
