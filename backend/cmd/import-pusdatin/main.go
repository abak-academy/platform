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
		mode             = flag.String("mode", "", "required: dry-run, apply, verify, or rollback")
		sourcePath       = flag.String("source", "", "verified Pusdatin CSV path")
		expectedSHA      = flag.String("expected-sha256", "", "expected source SHA-256")
		aliasesPath      = flag.String("aliases", "docs/pusdatin-geographic-aliases.json", "geographic alias JSON path")
		resolutionsPath  = flag.String("resolutions", "", "optional duplicate-resolution JSON path")
		reviewedChecksum = flag.String("reviewed-checksum", "", "required for apply; checksum from reviewed dry-run")
		manifestPath     = flag.String("manifest", "", "manifest JSON path for verify or rollback")
		manifestOutPath  = flag.String("manifest-out", "", "optional apply manifest JSON output path")
		appEnv           = flag.String("app-env", envDefault("APP_ENV", "dev"), "config environment")
		configDir        = flag.String("config-dir", envDefault("CONFIG_DIR", "config/env"), "config directory")
	)
	flag.Parse()

	if *mode != "dry-run" && *mode != "apply" && *mode != "verify" && *mode != "rollback" {
		exitf("mode must be dry-run, apply, verify, or rollback")
	}
	if (*mode == "dry-run" || *mode == "apply") && (*sourcePath == "" || *expectedSHA == "") {
		exitf("source and expected-sha256 are required")
	}
	if *mode == "apply" && *reviewedChecksum == "" {
		exitf("apply requires reviewed-checksum from a reviewed dry-run")
	}
	if (*mode == "verify" || *mode == "rollback") && *manifestPath == "" {
		exitf("verify and rollback require manifest")
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

	repo := repository.New(pool)
	svc := service.NewWithStore(repo, repo, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	if *mode == "verify" || *mode == "rollback" {
		manifest := loadManifest(*manifestPath)
		if *mode == "verify" {
			if err := svc.VerifyPusdatinImport(ctx, manifest); err != nil {
				exitf("verify failed: %v", err)
			}
			encodeJSON(map[string]string{"status": "verified"})
			return
		}
		if err := svc.RollbackPusdatinImport(ctx, manifest); err != nil {
			exitf("rollback failed: %v", err)
		}
		encodeJSON(map[string]string{"status": "rolled_back"})
		return
	}

	source, err := os.Open(*sourcePath)
	if err != nil {
		exitf("open source: %v", err)
	}
	defer source.Close()

	aliases, err := loadAliases(*aliasesPath)
	if err != nil {
		exitf("load aliases: %v", err)
	}
	resolutions, err := loadResolutions(*resolutionsPath)
	if err != nil {
		exitf("load resolutions: %v", err)
	}

	opts := model.PusdatinTransformOptions{
		ExpectedSourceSHA256: *expectedSHA,
		Cities:               loadCityReferences(ctx, repo),
		Aliases:              aliases,
		Resolutions:          resolutions,
	}

	var report *model.PusdatinImportReport
	switch *mode {
	case "dry-run":
		report, err = svc.DryRunPusdatinImport(ctx, source, opts)
	case "apply":
		var manifest model.PusdatinImportManifest
		report, manifest, err = svc.ApplyPusdatinImportWithManifest(ctx, source, opts, *reviewedChecksum)
		if err == nil && *manifestOutPath != "" {
			writeJSONFile(*manifestOutPath, manifest)
		}
	}
	if err != nil {
		_ = json.NewEncoder(os.Stdout).Encode(report)
		exitf("%s failed: %v", *mode, err)
	}
	if len(report.Blockers) > 0 {
		_ = json.NewEncoder(os.Stdout).Encode(report)
		os.Exit(2)
	}
	encodeJSON(report)
}

func loadManifest(path string) model.PusdatinImportManifest {
	data, err := os.ReadFile(path)
	if err != nil {
		exitf("read manifest: %v", err)
	}
	var manifest model.PusdatinImportManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		exitf("decode manifest: %v", err)
	}
	return manifest
}

func writeJSONFile(path string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		exitf("encode manifest: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		exitf("write manifest: %v", err)
	}
}

func encodeJSON(value any) {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
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

func loadResolutions(path string) ([]model.PusdatinDuplicateResolution, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var resolutions []model.PusdatinDuplicateResolution
	if err := json.Unmarshal(data, &resolutions); err != nil {
		return nil, err
	}
	return resolutions, nil
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
