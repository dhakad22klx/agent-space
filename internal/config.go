// Package internal holds shared application constants.
package internal

// ConfigDir is the shared directory for config.yml and credentials.json.
// Local development uses the project root. Production packaging can change
// this constant when the installed configuration path is finalized.
const ConfigDir = "."
