# SENAI Courses Track

Aplicação local em Go para acompanhar cursos e turmas disponíveis no SENAI-SP.

## Funcionalidades atuais

- scraping de cursos, cidades, unidades e turmas do portal SENAI-SP;
- filtros e assinaturas persistidos em SQLite;
- atualização agendada das assinaturas;
- notificações e consulta pelo Telegram;
- dashboard web server-rendered com Templ;
- API HTTP para administrar filtros, cursos e configurações.

## Stack

- Go 1.25+
- Gorilla Mux
- Templ
- Goquery
- SQLite (`modernc.org/sqlite`)
- Robfig Cron

## Desenvolvimento

```sh
go mod tidy
go test ./...
go vet ./...
go build ./cmd/server
```

Para iniciar:

```sh
go run ./cmd/server
```

O servidor escuta atualmente em `:8020` e cria o banco local `courses.db` no diretório de execução.

## Dados locais e credenciais

O banco SQLite e os logs não são versionados. Tokens e IDs do Telegram são armazenados na tabela de configuração do banco local e não devem ser adicionados ao Git.

## Estado do projeto

Este é o primeiro baseline versionado. A arquitetura, configuração operacional, testes e segurança ainda serão formalizados nas próximas etapas.
