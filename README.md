# 🕵️ reposnusern (POC)

**reposnusern** er et verktøy for å analysere GitHub-repositorier i en organisasjon – med nysgjerrighet, struktur og en dæsj AI.

## 🎯 Ambisjon

Målet med dette prosjektet er å lage et fleksibelt og utvidbart analyseverktøy for utviklingsmiljøer som ønsker innsikt i kodebasen sin. Prosjektet utvikles stegvis:

### 1. Datainnhenting

- Henter metadata, språkbruk, Dockerfiles og dependency-filer fra alle repoer i en GitHub-organisasjon.
- Data lagres i en relasjonsdatabase (PostgreSQL).
- Kjøres periodisk (f.eks. via cron-jobb).

### Teknologier og oppsett

- 🧠 Språk: Go
- 🗃️ Database: PostgreSQL (sqlc brukt for typesikker tilgang)
- 📦 Strukturelt monorepo – men med tydelig inndeling

## 🧪 PoC-status

### 1. Datainnhenting

Proof-of-Concept bruker følgende:
- `go + sqlc + PostgreSQL` 
- GitHub-API med mellomlagring i JSON
- Støtte for:
  - Repo-metadata og språk
  - Dockerfiles og dependency-filer
  - CI-konfigurasjon, README og sikkerhetsfunksjoner

Dette gir et godt grunnlag for å bygge videre analyser, inkludert rammeverksdeteksjon basert på språk og filstruktur.


## 📁 Prosjektstruktur
```
reposnusern/
├── cmd/
│   ├── fetch/      # Henter og lagrer data fra GitHub
│   └── migrate/    # Importerer JSON-data til PostgreSQL
│
├── internal/
│   ├── fetcher/    # GitHub-klient og mellomlagring
│   ├── dbwriter/   # Analyse av Dockerfiles og dependencies
│   ├── storage/    # sqlc-basert tilgang til databasen
│   └── parser/     # Parsing av filer
│
├── db/
│   ├── queries/    # sqlc-spørringer
│   └── schema.sql  # PostgreSQL-schema
│
├── data/           # Midlertidige JSON-filer
├── sqlc.yaml       # sqlc-konfigurasjon
├── go.mod / go.sum # Go-moduldefinisjoner
├── Dockerfile      # Bygging og kjøring i container
└── README.md
```

## Kjøring

### Json henting

For å hente data fra GitHub må du angi organisasjonsnavn og et gyldig GitHub-token som miljøvariabler:

```
export ORG=navikt
export GITHUB_TOKEN=<din_token>
go run ./cmd/fetch
```

Alternativt
```
# Bygg containeren
podman build -t reposnusnern .

# Kjør med nødvendige miljøvariabler og bind-mount for å se utdata
podman run --rm \
  -e ORG=dinorg \
  -e GITHUB_TOKEN=ghp_dintokenher \
  -e REPOSNUSERDEBUG=true \
  -v "$PWD/data":/data \
  reposnusnern

```

Dette scriptet vil:
- en rå oversikt over alle repoer (data/navikt_repos_raw_dump.json)
- detaljert analyse av ikke-arkiverte repoer (data/navikt_analysis_data.json)

Merk: GitHub har en grense på 5000 API-kall per time for autentiserte brukere. Koden håndterer dette automatisk ved å pause og fortsette når grensen er nådd.

### Migrering til PostgresSQL

Eksempel:

```
export POSTGRES_DSN="postgres://<bruker>:<passord>@<fqdn>:5432/reposnusern?sslmode=require"
go run ./cmd/migrate
```

## TODO

- [ ] Parsing av forskjellige dependency filer
- [ ] Også hente REST API endpoints for software bill of materials (SBOM)
- [ ] 🔐 Hindre at passord og secrets utilsiktet havner i logger
- [ ] ☁️ Gjøre klart for K8s-deploy (config, secrets, jobs)
- [ ] ✅ Legge til noen enkle tester (det var jo bare en PoC 😅)
- [ ] 🧹 Refaktorering og deling av logikk
- [ ] Oppdatere schema så vi tar vare på dato vi har hentet informasjonen fra. (Så vi kan ta vare på trenden.)

## Annen inspirasjon
 - [Fuck it, ship it - Stine Mølgaard og Jacob Bøtter](https://fuckitshipit.dk/)
 - [Codin' Dirty - Carson Gross](https://htmx.org/essays/codin-dirty/)

## 🤖 Erklæring om bruk av generativ KI

Under utviklingen av dette innholdet har forfatter(e) benyttet generativ KI – inkludert M365 Copilot og ChatGPT – til å omformulere og effektivisere tekst og kode. Alt innhold er deretter gjennomgått og en del redigert manuelt. 
