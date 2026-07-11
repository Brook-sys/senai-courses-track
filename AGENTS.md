# SENAI Courses Track — Agent Guide

## Objetivo

Aplicação Go local para rastrear cursos e turmas do SENAI-SP, persistir assinaturas e avisar o usuário pelo dashboard e Telegram.

## Estrutura

- `cmd/server/`: bootstrap HTTP, scheduler e Telegram.
- `internal/api/`: rotas e handlers HTTP.
- `internal/api/views/`: templates Templ e código gerado.
- `internal/scraper/`: integração e parsing do portal SENAI-SP.
- `internal/storage/`: SQLite e persistência.
- `internal/scheduler/`: atualização programada de assinaturas.
- `internal/notifier/`: notificações Telegram.
- `internal/telegrambot/`: interação via bot.

## Regras

1. Não versione `courses.db`, logs, tokens, chat IDs ou outros dados locais.
2. Preserve o sistema como aplicação Go simples e leve; evite frameworks pesados.
3. Mudanças no scraper devem tratar alterações ou respostas inesperadas do site remoto de forma resiliente.
4. Adicione testes para parsing, persistência e regras de negócio sempre que alterar comportamento.
5. Não edite manualmente `*_templ.go` quando a origem correspondente for um arquivo `.templ`; regenere com Templ.
6. Antes de concluir mudanças, rode:

```sh
go test ./...
go vet ./...
go build ./cmd/server
```

7. Faça commits pequenos e claros e envie para `origin/main` quando solicitado.
