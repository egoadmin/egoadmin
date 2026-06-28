package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	configPath          = ".egoadmin/template.json"
	defaultTemplateRepo = "https://github.com/egoadmin/egoadmin.git"
)

type templateConfig struct {
	Name       string   `json:"name"`
	Slug       string   `json:"slug"`
	Module     string   `json:"module"`
	EnvPrefix  string   `json:"envPrefix"`
	GoPackage  string   `json:"goPackage"`
	Services   []string `json:"services"`
	SkipPaths  []string `json:"skipPaths,omitempty"`
	UpdateTime string   `json:"-"`
}

type identity struct {
	Name      string
	Slug      string
	Module    string
	EnvPrefix string
	GoPackage string
}

type replacement struct {
	Old string `json:"old"`
	New string `json:"new"`
}

type replacementFlags []replacement

func (r *replacementFlags) String() string {
	if r == nil {
		return ""
	}
	values := make([]string, 0, len(*r))
	for _, item := range *r {
		values = append(values, item.Old+"="+item.New)
	}
	return strings.Join(values, ",")
}

func (r *replacementFlags) Set(value string) error {
	oldValue, newValue, ok := strings.Cut(value, "=")
	if !ok || oldValue == "" {
		return fmt.Errorf("replacement must use old=new format: %q", value)
	}
	*r = append(*r, replacement{Old: oldValue, New: newValue})
	return nil
}

type renameOptions struct {
	Root           string
	From           identity
	To             identity
	Services       []string
	Extra          []replacement
	Write          bool
	IncludeAgents  bool
	UpdateConfig   bool
	PrintUnchanged bool
}

type fileChange struct {
	Path        string
	Occurrences int
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stderr)
		return errors.New("missing subcommand")
	}

	switch args[0] {
	case "rename":
		return runRename(args[1:], stdout, stderr)
	case "init":
		return runInit(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  egoadminctl rename [flags]
  egoadminctl init [flags]

Subcommands:
  rename    Rename the current template project in place. Dry-run by default.
  init      Clone a template repository into a destination and rename it.

Run "egoadminctl <subcommand> -h" for flags.`)
}

func runRename(args []string, stdout, stderr io.Writer) error {
	var extra replacementFlags
	var services string
	opts := renameOptions{Root: ".", UpdateConfig: true}

	fs := flag.NewFlagSet("rename", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addIdentityFlags(fs, &opts.From, "from")
	addIdentityFlags(fs, &opts.To, "")
	fs.StringVar(&opts.Root, "root", ".", "project root to rename")
	fs.StringVar(&services, "services", "", "comma-separated service names; defaults to template config services")
	fs.Var(&extra, "replace", "extra replacement in old=new format; may be repeated")
	fs.BoolVar(&opts.Write, "write", false, "write changes; default is dry-run")
	fs.BoolVar(&opts.IncludeAgents, "include-agents", false, "also rewrite .agents and .claude agent documentation")
	fs.BoolVar(&opts.UpdateConfig, "update-config", true, "write .egoadmin/template.json with the target identity")

	if err := fs.Parse(args); err != nil {
		return err
	}
	opts.Extra = extra
	opts.Services = parseServices(services)

	result, err := renameProject(opts)
	if err != nil {
		return err
	}
	printRenameResult(stdout, opts.Write, result)
	return nil
}

func runInit(args []string, stdout, stderr io.Writer) error {
	var extra replacementFlags
	var services string
	var repo, dest, branch string
	var depth int
	var dryRun, keepGit, includeAgents bool
	var from, to identity

	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addIdentityFlags(fs, &from, "from")
	addIdentityFlags(fs, &to, "")
	fs.StringVar(&repo, "repo", defaultTemplateRepo, "template git repository URL")
	fs.StringVar(&dest, "dest", "", "destination directory")
	fs.StringVar(&branch, "branch", "", "git branch, tag, or ref to clone")
	fs.IntVar(&depth, "depth", 1, "git clone depth; set 0 for full clone")
	fs.StringVar(&services, "services", "", "comma-separated service names; defaults to template config services")
	fs.Var(&extra, "replace", "extra replacement in old=new format; may be repeated")
	fs.BoolVar(&dryRun, "dry-run", false, "print planned clone and rename without writing")
	fs.BoolVar(&keepGit, "keep-git", false, "keep the cloned .git directory")
	fs.BoolVar(&includeAgents, "include-agents", false, "also rewrite .agents and .claude agent documentation")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if dest == "" {
		return errors.New("--dest is required")
	}
	if err := validateTarget(to); err != nil {
		return err
	}

	cloneArgs := gitCloneArgs(repo, dest, branch, depth)
	if dryRun {
		fmt.Fprintf(stdout, "dry-run: git %s\n", strings.Join(cloneArgs, " "))
		return nil
	}
	if err := ensureDestinationAvailable(dest); err != nil {
		return err
	}

	cmd := exec.Command("git", cloneArgs...)
	cmd.Stdout = stderr
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	if !keepGit {
		if err := os.RemoveAll(filepath.Join(dest, ".git")); err != nil {
			return fmt.Errorf("remove cloned .git directory: %w", err)
		}
	}

	opts := renameOptions{
		Root:          dest,
		From:          from,
		To:            to,
		Services:      parseServices(services),
		Extra:         extra,
		Write:         true,
		IncludeAgents: includeAgents,
		UpdateConfig:  true,
	}
	result, err := renameProject(opts)
	if err != nil {
		return err
	}
	printRenameResult(stdout, true, result)
	fmt.Fprintf(stdout, "next: cd %s && go mod tidy && make gen\n", dest)
	return nil
}

func addIdentityFlags(fs *flag.FlagSet, id *identity, prefix string) {
	name := "name"
	slug := "slug"
	module := "module"
	envPrefix := "env-prefix"
	goPackage := "go-package"
	if prefix != "" {
		name = prefix + "-name"
		slug = prefix + "-slug"
		module = prefix + "-module"
		envPrefix = prefix + "-env-prefix"
		goPackage = prefix + "-go-package"
	}
	fs.StringVar(&id.Name, name, "", "project display name")
	fs.StringVar(&id.Slug, slug, "", "lowercase project slug used in service names and local credentials")
	fs.StringVar(&id.Module, module, "", "Go module path")
	fs.StringVar(&id.EnvPrefix, envPrefix, "", "environment variable prefix")
	fs.StringVar(&id.GoPackage, goPackage, "", "root Go package name")
}

func renameProject(opts renameOptions) ([]fileChange, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, err
	}

	cfg, _ := loadTemplateConfig(root)
	opts.From = fillIdentity(opts.From, cfg)
	opts.To = normalizeIdentity(opts.To)
	if len(opts.Services) == 0 {
		opts.Services = cfg.Services
	}
	if len(opts.Services) == 0 {
		opts.Services = []string{"gateway", "user"}
	}
	if err := validateIdentity("source", opts.From); err != nil {
		return nil, err
	}
	if err := validateTarget(opts.To); err != nil {
		return nil, err
	}

	replacements := buildReplacements(opts.From, opts.To, opts.Services, opts.Extra)

	changes, err := rewriteFiles(root, replacements, opts)
	if err != nil {
		return nil, err
	}
	if opts.UpdateConfig {
		configChange, err := updateTemplateConfig(root, opts.To, opts.Services, opts.Write)
		if err != nil {
			return nil, err
		}
		if configChange != nil {
			changes = append(changes, *configChange)
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Path < changes[j].Path
	})
	return changes, nil
}

func fillIdentity(id identity, cfg templateConfig) identity {
	if id.Name == "" {
		id.Name = cfg.Name
	}
	if id.Slug == "" {
		id.Slug = cfg.Slug
	}
	if id.Module == "" {
		id.Module = cfg.Module
	}
	if id.EnvPrefix == "" {
		id.EnvPrefix = cfg.EnvPrefix
	}
	if id.GoPackage == "" {
		id.GoPackage = cfg.GoPackage
	}
	return normalizeIdentity(id)
}

func normalizeIdentity(id identity) identity {
	id.Slug = strings.ToLower(strings.TrimSpace(id.Slug))
	id.Module = strings.TrimSpace(id.Module)
	id.Name = strings.TrimSpace(id.Name)
	if id.GoPackage == "" {
		id.GoPackage = sanitizeGoPackage(id.Slug)
	}
	if id.EnvPrefix == "" {
		id.EnvPrefix = envPrefixFromSlug(id.Slug)
	}
	id.EnvPrefix = strings.ToUpper(strings.TrimSpace(id.EnvPrefix))
	return id
}

func validateIdentity(label string, id identity) error {
	if id.Name == "" {
		return fmt.Errorf("%s project name is required", label)
	}
	if id.Slug == "" {
		return fmt.Errorf("%s project slug is required", label)
	}
	if id.Module == "" {
		return fmt.Errorf("%s module path is required", label)
	}
	if id.EnvPrefix == "" {
		return fmt.Errorf("%s env prefix is required", label)
	}
	if id.GoPackage == "" {
		return fmt.Errorf("%s Go package is required", label)
	}
	return nil
}

func validateTarget(id identity) error {
	id = normalizeIdentity(id)
	if err := validateIdentity("target", id); err != nil {
		return err
	}
	if strings.ContainsAny(id.Slug, " \t\r\n") {
		return fmt.Errorf("target slug must not contain whitespace: %q", id.Slug)
	}
	if !isValidGoPackage(id.GoPackage) {
		return fmt.Errorf("target Go package %q is not a valid package identifier", id.GoPackage)
	}
	return nil
}

func buildReplacements(from, to identity, services []string, extra []replacement) []replacement {
	var items []replacement
	add := func(oldValue, newValue string) {
		if oldValue == "" || oldValue == newValue {
			return
		}
		items = append(items, replacement{Old: oldValue, New: newValue})
	}

	for _, item := range extra {
		add(item.Old, item.New)
	}
	add(from.Module, to.Module)
	add(from.EnvPrefix, to.EnvPrefix)

	for _, service := range services {
		service = strings.TrimSpace(service)
		if service == "" {
			continue
		}
		add(from.Slug+"-"+service, to.Slug+"-"+service)
		add(from.Slug+"_"+service, to.Slug+"_"+service)
	}
	add(from.Slug+"-local", to.Slug+"-local")
	add(from.Slug+"_local", to.Slug+"_local")
	add(from.Name, to.Name)
	add(strings.ToUpper(from.Slug), strings.ToUpper(to.Slug))
	add(from.GoPackage, to.GoPackage)
	add(from.Slug, to.Slug)

	sort.SliceStable(items, func(i, j int) bool {
		return len(items[i].Old) > len(items[j].Old)
	})
	return dedupeReplacements(items)
}

func dedupeReplacements(items []replacement) []replacement {
	seen := make(map[string]bool, len(items))
	result := make([]replacement, 0, len(items))
	for _, item := range items {
		if seen[item.Old] {
			continue
		}
		seen[item.Old] = true
		result = append(result, item)
	}
	return result
}

func rewriteFiles(root string, replacements []replacement, opts renameOptions) ([]fileChange, error) {
	var changes []fileChange
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if entry.IsDir() {
			if shouldSkipDir(rel, opts.IncludeAgents) {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldSkipFile(rel) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > 10*1024*1024 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !isText(data) {
			return nil
		}
		next, count := applyReplacements(data, replacements)
		if count == 0 {
			return nil
		}
		if opts.Write {
			if err := os.WriteFile(path, next, info.Mode().Perm()); err != nil {
				return err
			}
		}
		changes = append(changes, fileChange{Path: rel, Occurrences: count})
		return nil
	})
	return changes, err
}

func applyReplacements(data []byte, replacements []replacement) ([]byte, int) {
	next := data
	count := 0
	for _, item := range replacements {
		oldBytes := []byte(item.Old)
		occurrences := bytes.Count(next, oldBytes)
		if occurrences == 0 {
			continue
		}
		next = bytes.ReplaceAll(next, oldBytes, []byte(item.New))
		count += occurrences
	}
	return next, count
}

func shouldSkipDir(rel string, includeAgents bool) bool {
	if rel == ".git" || rel == ".idea" || rel == ".vscode" {
		return true
	}
	if (rel == ".agents" || rel == ".claude") && !includeAgents {
		return true
	}
	for _, prefix := range []string{
		"api/gen",
		"api/openapi",
		"api/catalog",
		"web/dist",
		"web/node_modules",
		"node_modules",
		"vendor",
		"tools/egoadminctl",
		"dist",
		"logs",
		"_output",
		"tmp",
		"bin",
		"deploy/data",
	} {
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
			return true
		}
	}
	if strings.HasPrefix(rel, "test/data/") {
		return true
	}
	return false
}

func shouldSkipFile(rel string) bool {
	if rel == configPath {
		return true
	}
	return false
}

func isText(data []byte) bool {
	if bytes.Contains(data, []byte{0}) {
		return false
	}
	return utf8.Valid(data)
}

func loadTemplateConfig(root string) (templateConfig, error) {
	var cfg templateConfig
	data, err := os.ReadFile(filepath.Join(root, configPath))
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", configPath, err)
	}
	return cfg, nil
}

func updateTemplateConfig(root string, id identity, services []string, write bool) (*fileChange, error) {
	id = normalizeIdentity(id)
	cfg := templateConfig{
		Name:      id.Name,
		Slug:      id.Slug,
		Module:    id.Module,
		EnvPrefix: id.EnvPrefix,
		GoPackage: id.GoPackage,
		Services:  services,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	path := filepath.Join(root, configPath)
	old, readErr := os.ReadFile(path)
	if readErr == nil && bytes.Equal(old, data) {
		return nil, nil
	}
	if write {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return nil, err
		}
	}
	return &fileChange{Path: configPath, Occurrences: 1}, nil
}

func printRenameResult(w io.Writer, write bool, changes []fileChange) {
	mode := "dry-run"
	if write {
		mode = "write"
	}
	fmt.Fprintf(w, "%s: %d files would change\n", mode, len(changes))
	if write {
		fmt.Fprintf(w, "%s: %d files changed\n", mode, len(changes))
	}
	for _, change := range changes {
		fmt.Fprintf(w, "  %s (%d)\n", change.Path, change.Occurrences)
	}
}

func parseServices(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	services := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			services = append(services, part)
		}
	}
	return services
}

func gitCloneArgs(repo, dest, branch string, depth int) []string {
	args := []string{"clone"}
	if depth > 0 {
		args = append(args, "--depth", fmt.Sprint(depth))
	}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repo, dest)
	return args
}

func ensureDestinationAvailable(dest string) error {
	info, err := os.Stat(dest)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("destination exists and is not a directory: %s", dest)
	}
	entries, err := os.ReadDir(dest)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("destination directory is not empty: %s", dest)
	}
	return nil
}

func sanitizeGoPackage(slug string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(slug) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	pkg := b.String()
	if pkg == "" {
		return "app"
	}
	first, _ := utf8.DecodeRuneInString(pkg)
	if unicode.IsDigit(first) {
		return "app" + pkg
	}
	return pkg
}

func isValidGoPackage(pkg string) bool {
	if pkg == "" {
		return false
	}
	for i, r := range pkg {
		if i == 0 && !unicode.IsLetter(r) && r != '_' {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func envPrefixFromSlug(slug string) string {
	var b strings.Builder
	for _, r := range slug {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	if b.Len() == 0 {
		return "APP"
	}
	return b.String()
}
