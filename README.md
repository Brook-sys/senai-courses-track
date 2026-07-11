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

## Docker

A imagem oficial é publicada no GitHub Container Registry:

```sh
docker pull ghcr.io/brook-sys/senai-courses-track:latest
```

Execução direta com persistência:

```sh
docker run -d \
  --name senai-courses-track \
  --restart unless-stopped \
  -p 8020:8020 \
  -v senai-courses-data:/data \
  ghcr.io/brook-sys/senai-courses-track:latest
```

Ou com Compose:

```sh
docker compose up -d
docker compose ps
docker compose logs -f
```

O dashboard fica disponível em `http://localhost:8020` e o healthcheck em
`http://localhost:8020/healthz`.

### Variáveis de ambiente

| Variável | Padrão no container | Descrição |
| --- | --- | --- |
| `SENAI_TRACK_ADDR` | `:8020` | Endereço HTTP do servidor. |
| `SENAI_TRACK_DB_PATH` | `/data/courses.db` | Caminho persistente do SQLite. |
| `TZ` | `America/Sao_Paulo` | Fuso horário do container. |

O container roda sem privilégios como UID/GID `10001` e persiste os dados no
volume `/data`.

## Publicação no GHCR

O workflow `.github/workflows/container.yml` executa formatação, testes, vet e
build. Em pushes para `main` e tags `v*`, publica imagens multi-arquitetura para
`linux/amd64` e `linux/arm64`, com tags `latest`, branch, SHA e versão.

## Dados locais e credenciais

O banco SQLite e os logs não são versionados. Tokens e IDs do Telegram são armazenados na tabela de configuração do banco local e não devem ser adicionados ao Git.

## Estado do projeto

Este é o primeiro baseline versionado. A arquitetura, configuração operacional, testes e segurança ainda serão formalizados nas próximas etapas.
