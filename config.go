// config.go — runtime configuration for the buildwheels tool.
package main

const defaultPyPIURL = "https://upload.pypi.org/legacy/"

// Config holds all runtime configuration, populated from CLI flags.
// Fields that are empty at parse-time are derived from Repo in main.
type Config struct {
	// GitHub source
	Repo    string // "owner/name"
	Version string // release tag; "" means latest

	// Package identity — derived from Repo/BinaryName when left empty
	BinaryName  string // binary filename inside archives
	PackageName string // Python package name
	EntryPoint  string // console_scripts entry point
	Summary     string // one-line PyPI description
	LicenseExpr string // SPDX expression, e.g. "MIT"

	// Build
	PyVersion  string   // Python package version; mirrors Version when ""
	Output     string   // output directory for .whl files
	Platforms  []string // empty = all supported platforms
	AssetNames []string // explicit asset filenames, overrides auto-detect

	// PyPI upload
	Upload   bool
	PyPIURL  string
	PyPIUser string

	// Input files
	LicensePath     string // "" = fetch from repo
	DescriptionPath string // "" = DESCRIPTION.md

	// Cache & logging
	CacheDir string // "" = disable caching
	Debug    bool
}
