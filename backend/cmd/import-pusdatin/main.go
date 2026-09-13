package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"akademi-bimbel/config"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
)

func main() {
	var (
		mode             = flag.String("mode", "", "required: dry-run or apply")
		sourcePath       = flag.String("source", "", "verified Pusdatin CSV path")
		expectedSHA      = flag.String("expected-sha256", "", "expected source SHA-256")
		aliasesPath      = flag.String("aliases", "docs/pusdatin-geographic-aliases.json", "geographic alias JSON path")
		reviewedChecksum = flag.String("reviewed-checksum", "", "required for apply; checksum from reviewed dry-run")
		appEnv           = flag.String("app-env", envDefault("APP_ENV", "dev"), "config environment")
		configDir        = flag.String("config-dir", envDefault("CONFIG_DIR", "config/env"), "config directory")
	)
	flag.Parse()

	if *mode != "dry-run" && *mode != "apply" {
		exitf("mode must be dry-run or apply")
	}
	if *sourcePath == "" || *expectedSHA == "" {
		exitf("source and expected-sha256 are required")
	}
	if *mode == "apply" && *reviewedChecksum == "" {
		exitf("apply requires reviewed-checksum from a reviewed dry-run")
	}

	cfg, err := config.Load(*appEnv, *configDir)
	if err != nil {
		exitf("load config: %v", err)
	}
	ctx := context.Background()
	pool, err := infra.NewPoolWithMaxConns(ctx, cfg.DatabaseURL, 1)
	if err != nil {
		exitf("connect database: %v", err)
	}
	defer pool.Close()

	source, err := os.Open(*sourcePath)
	if err != nil {
		exitf("open source: %v", err)
	}
	defer source.Close()

	aliases, err := loadAliases(*aliasesPath)
	if err != nil {
		exitf("load aliases: %v", err)
	}

	repo := repository.New(pool)
	svc := service.NewWithStore(repo, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	opts := model.PusdatinTransformOptions{
		ExpectedSourceSHA256: *expectedSHA,
		Cities:               loadCityReferences(ctx, repo),
		Aliases:              aliases,
	}

	var report *model.PusdatinImportReport
	switch *mode {
	case "dry-run":
		report, err = svc.DryRunPusdatinImport(ctx, source, opts)
	case "apply":
		report, err = svc.ApplyPusdatinImport(ctx, source, opts, *reviewedChecksum)
	}
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(report)
		exitf("%s failed: %v", *mode, err)
	}
	if len(report.Blockers) > 0 {
		_ = json.NewEncoder(os.Stdout).Encode(report)
		os.Exit(2)
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		exitf("encode report: %v", err)
	}
}

func loadCityReferences(ctx context.Context, repo *repository.Repository) []model.PusdatinCityReference {
	rows, err := repo.Pool().Query(ctx,
		`SELECT c.id, c.name, p.id, p.name FROM city c JOIN province p ON p.id = c.province_id`,
	)
	if err != nil {
		exitf("load city references: %v", err)
	}
	defer rows.Close()

	var out []model.PusdatinCityReference
	for rows.Next() {
		var city model.PusdatinCityReference
		if err := rows.Scan(&city.ID, &city.Name, &city.ProvinceID, &city.ProvinceName); err != nil {
			exitf("scan city references: %v", err)
		}
		out = append(out, city)
	}
	if err := rows.Err(); err != nil {
		exitf("read city references: %v", err)
	}
	return out
}

func loadAliases(path string) ([]model.PusdatinGeographicAlias, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var aliases []model.PusdatinGeographicAlias
	if err := json.Unmarshal(data, &aliases); err != nil {
		return nil, err
	}
	return aliases, nil
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
