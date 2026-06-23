package app

// This file defines service interfaces for future HTTP API use.
// CLI currently uses concrete *AppService types from each domain package.

// The architecture ensures HTTP API layer can reuse the same services:
//
//   ProjectAppService = project.AppService
//   ServiceAppService = service.AppService
//   RouteAppService   = route.AppService
//   ApplyAppService   = apply.AppService
//   HealthAppService  = health.AppService
//   LogAppService     = logs.AppService
//
// Future: define interfaces here and have HTTP handlers depend on interfaces.

// SettingsData holds read-only settings for the CLI.
type SettingsData struct {
	Provider      string
	CaddyfilePath string
	CaddyBinary   string
	ReloadCommand string
	BackupDir     string
	SQLitePath    string
	ConfigDir     string
	DataDir       string
}
