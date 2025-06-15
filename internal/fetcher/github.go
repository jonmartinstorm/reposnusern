package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jonmartinstorm/reposnusern/internal/config"
	"github.com/jonmartinstorm/reposnusern/internal/models"
)

type GitHubAPIClient struct{}

type GitHubClient interface {
	GetRepoPage(cfg config.Config, page int) ([]models.RepoMeta, error)
}

type GraphQLFetcher interface {
	Fetch(owner, name, token string, baseRepo models.RepoMeta) *models.RepoEntry
}

// Kombinerer begge i én (valgfritt, men ryddig hvis du bruker begge via én struct)
type GitHubAPI interface {
	GitHubClient
	GraphQLFetcher
}

type TreeFile struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	Type string `json:"type"`
}

// Injecter en klient (for testbarhet)
var HttpClient = http.DefaultClient

func DoRequestWithRateLimit(method, url, token string, body []byte, out interface{}) error {
	for {
		slog.Info("Henter URL", "url", url)

		req, err := http.NewRequestWithContext(context.Background(), method, url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")
		if method == "POST" {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := HttpClient.Do(req)
		if err != nil {
			return err
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.Printf("advarsel: klarte ikke å lukke body: %v", err)
			}
		}()

		if rl := resp.Header.Get("X-RateLimit-Remaining"); rl == "0" {
			reset := resp.Header.Get("X-RateLimit-Reset")
			if ts, err := strconv.ParseInt(reset, 10, 64); err == nil {
				wait := time.Until(time.Unix(ts, 0)) + time.Second
				slog.Warn("Rate limit nådd", "venter", wait.Truncate(time.Second))
				time.Sleep(wait)
				continue
			}
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			bodyBytes, _ := io.ReadAll(resp.Body)
			slog.Error("GitHub-feil", "status", resp.StatusCode, "body", string(bodyBytes))
			return fmt.Errorf("GitHub API-feil: status %d – %s", resp.StatusCode, string(bodyBytes))
		}

		return json.NewDecoder(resp.Body).Decode(out)
	}
}

func (g *GitHubAPIClient) GetRepoPage(cfg config.Config, page int) ([]models.RepoMeta, error) {
	url := fmt.Sprintf("https://api.github.com/orgs/%s/repos?per_page=100&type=all&page=%d", cfg.Org, page)
	var pageRepos []models.RepoMeta
	slog.Info("Henter repos", "page", page)

	err := DoRequestWithRateLimit("GET", url, cfg.Token, nil, &pageRepos)
	if err != nil {
		return nil, err
	}

	return pageRepos, nil
}

func (g *GitHubAPIClient) Fetch(owner, name, token string, baseRepo models.RepoMeta) *models.RepoEntry {
	return FetchRepoGraphQL(owner, name, token, baseRepo)
}

func FetchRepoGraphQL(owner, name, token string, baseRepo models.RepoMeta) *models.RepoEntry {
	query := BuildRepoQuery(owner, name)

	reqBody := map[string]string{"query": query}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		slog.Error("Kunne ikke serialisere GraphQL-request", "repo", owner+"/"+name, "error", err)
		return nil
	}

	var result map[string]interface{}
	err = DoRequestWithRateLimit("POST", "https://api.github.com/graphql", token, bodyBytes, &result)
	if err != nil {
		slog.Error("GraphQL-kall feilet", "repo", owner+"/"+name, "error", err)
		return nil
	}

	if errs, ok := result["errors"]; ok {
		slog.Warn("GraphQL-resultat har feil", "repo", owner+"/"+name, "errors", errs)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok || data["repository"] == nil {
		slog.Warn("Ingen repository-data fra GraphQL", "repo", owner+"/"+name)
		return nil
	}

	sbom := FetchSBOM(owner, name, token)
	entry := ParseRepoData(data, baseRepo)

	entry.SBOM = sbom
	return entry
}

func FetchSBOM(owner, repo, token string) map[string]interface{} {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/dependency-graph/sbom", owner, repo)

	var sbom map[string]interface{}
	err := DoRequestWithRateLimit("GET", url, token, nil, &sbom)
	if err != nil {
		slog.Warn("SBOM-kall feilet", "repo", owner+"/"+repo, "error", err)
		return nil
	}
	return sbom
}

func ParseRepoData(data map[string]interface{}, baseRepo models.RepoMeta) *models.RepoEntry {
	repoData, ok := data["repository"].(map[string]interface{})
	if !ok {
		slog.Warn("Mangler 'repository'-data i GraphQL-response")
		return nil
	}

	updatedRepo := baseRepo
	updatedRepo.Readme = ExtractReadme(repoData)
	updatedRepo.Security = ExtractSecurity(repoData)

	return &models.RepoEntry{
		Repo:      updatedRepo,
		Languages: ExtractLanguages(repoData),
		Files:     ExtractFiles(repoData),
		CIConfig:  ExtractCI(repoData),
	}
}

func ExtractLanguages(data map[string]interface{}) map[string]int {
	langs := map[string]int{}

	if langsData, ok := data["languages"].(map[string]interface{}); ok {
		if edges, ok := langsData["edges"].([]interface{}); ok {
			for _, edgeRaw := range edges {
				edge, ok := edgeRaw.(map[string]interface{})
				if !ok {
					continue
				}

				// node["name"]
				var name string
				if node, ok := edge["node"].(map[string]interface{}); ok {
					name, _ = node["name"].(string)
				}

				// size
				var size int
				if s, ok := edge["size"].(float64); ok {
					size = int(s)
				}

				if name != "" && size > 0 {
					langs[name] = size
				}
			}
		}
	}
	return langs
}

func ExtractFiles(data map[string]interface{}) map[string][]models.FileEntry {
	files := map[string][]map[string]string{}

	// Dependency files
	if deps, ok := data["dependencies"].(map[string]interface{}); ok {
		if entries, ok := deps["entries"].([]interface{}); ok {
			for _, raw := range entries {
				entry, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}

				name, _ := entry["name"].(string)
				lowerName := strings.ToLower(name)

				if !strings.Contains(lowerName, "dockerfile") {
					continue
				}

				var content string
				if obj, ok := entry["object"].(map[string]interface{}); ok {
					if text, ok := obj["text"].(string); ok {
						content = text
					}
				}

				if content != "" {
					files[lowerName] = append(files[lowerName], map[string]string{
						"path":    name,
						"content": content,
					})
				}
			}
		}
	}
	return ConvertFiles(files)
}

func ExtractCI(data map[string]interface{}) []models.FileEntry {
	ci := []map[string]string{}
	// CI config
	if workflows, ok := data["workflows"].(map[string]interface{}); ok {
		if entries, ok := workflows["entries"].([]interface{}); ok {
			for _, raw := range entries {
				entry, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := entry["name"].(string)

				// Hent .object.text hvis det finnes og er string
				var content string
				if obj, ok := entry["object"].(map[string]interface{}); ok {
					if text, ok := obj["text"].(string); ok {
						content = text
					}
				}

				// Bare legg til hvis det finnes
				if content != "" {
					ci = append(ci, map[string]string{
						"path":    ".github/workflows/" + name,
						"content": content,
					})
				}
			}
		}
	}
	return ConvertToFileEntries(ci)
}

func ExtractSecurity(data map[string]interface{}) map[string]bool {
	security := map[string]bool{}
	security["has_security_md"] = data["SECURITY"] != nil
	security["has_dependabot"] = data["dependabot"] != nil
	security["has_codeql"] = data["codeql"] != nil
	return security
}

func ExtractReadme(data map[string]interface{}) string {
	if val, ok := data["README"].(map[string]interface{}); ok {
		if text, ok := val["text"].(string); ok {
			return text
		}
	}
	return ""
}

func BuildRepoQuery(owner string, name string) string {
	query := fmt.Sprintf(`
	{
		repository(owner: "%s", name: "%s") {
			defaultBranchRef {
				name
			}
			README: object(expression: "HEAD:README.md") {
				... on Blob {
					text
				}
			}
			SECURITY: object(expression: "HEAD:SECURITY.md") {
				... on Blob {
					text
				}
			}
			dependabot: object(expression: "HEAD:.github/dependabot.yml") {
				... on Blob {
					text
				}
			}
			codeql: object(expression: "HEAD:.github/codeql.yml") {
				... on Blob {
					text
				}
			}
			workflows: object(expression: "HEAD:.github/workflows") {
				... on Tree {
					entries {
						name
						object {
							... on Blob {
								text
							}
						}
					}
				}
			}
			dependencies: object(expression: "HEAD:") {
				... on Tree {
					entries {
						name
						object {
							... on Blob {
								text
							}
						}
					}
				}
			}
			languages(first: 10) {
				edges {
					size
					node {
						name
					}
				}
			}
		}
	}`, owner, name)
	return query
}

func ConvertToFileEntries(entries []map[string]string) []models.FileEntry {
	var result []models.FileEntry
	for _, e := range entries {
		result = append(result, models.FileEntry{
			Path:    e["path"],
			Content: e["content"],
		})
	}
	return result
}

func ConvertFiles(input map[string][]map[string]string) map[string][]models.FileEntry {
	out := map[string][]models.FileEntry{}
	for k, v := range input {
		out[k] = ConvertToFileEntries(v)
	}
	return out
}
