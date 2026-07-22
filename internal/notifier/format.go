package notifier

import (
	"fmt"
	"html"
	"strings"

	"github.com/Brook-sys/senai-courses-track/internal/scraper"
)

func formatCourseMessage(c scraper.Course, subName string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("🔔 Nova oportunidade em: <b>%s</b>\n\n", html.EscapeString(subName)))
	sb.WriteString(fmt.Sprintf("📚 <b>%s</b>\n", html.EscapeString(c.Title)))

	if c.Duration != "" {
		sb.WriteString(fmt.Sprintf("⏱️ Carga horária: %s\n", html.EscapeString(c.Duration)))
	}
	sb.WriteString(fmt.Sprintf("🔗 <a href=\"%s\">Abrir curso</a>\n", html.EscapeString(c.URL)))

	for _, turma := range c.Turmas {
		sb.WriteString("\n")
		if turma.Unit != "" {
			sb.WriteString(fmt.Sprintf("🏫 Unidade: %s\n", html.EscapeString(turma.Unit)))
		}
		if turma.Address != "" {
			sb.WriteString(fmt.Sprintf("📍 Endereço: %s\n", html.EscapeString(turma.Address)))
		}
		if turma.Phone != "" {
			sb.WriteString(fmt.Sprintf("☎️ Telefone: %s\n", html.EscapeString(turma.Phone)))
		}
		if turma.Vacancies != "" {
			sb.WriteString(fmt.Sprintf("🎟️ Vagas: %s\n", html.EscapeString(turma.Vacancies)))
		}
		if turma.StartDate != "" || turma.EndDate != "" {
			sb.WriteString(fmt.Sprintf("📅 Início/Fim: %s até %s\n", html.EscapeString(turma.StartDate), html.EscapeString(turma.EndDate)))
		}
		if turma.Period != "" {
			sb.WriteString(fmt.Sprintf("🗓️ Dias: %s\n", html.EscapeString(turma.Period)))
		}
		if turma.Schedule != "" {
			sb.WriteString(fmt.Sprintf("🕒 Horário: %s\n", html.EscapeString(turma.Schedule)))
		}
	}

	return sb.String()
}
